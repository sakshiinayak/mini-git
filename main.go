package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func writeObject(objectType string, content []byte) string {
	header := fmt.Sprintf("%s %d\u0000", objectType, len(content))
	object := append([]byte(header), content...)

	h := sha1.Sum(object)
	hash := fmt.Sprintf("%x", h)

	dir := ".git/objects/" + hash[:2]
	file := dir + "/" + hash[2:]

	os.MkdirAll(dir, 0755)
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating object file: %s\n", err)
		os.Exit(1)
	}
	defer f.Close()

	zw := zlib.NewWriter(f)
	_, _ = zw.Write(object)
	zw.Close()

	return hash
}

func readObject(hash string) (string, []byte) {
	dir := ".git/objects/" + hash[:2]
	file := dir + "/" + hash[2:]

	f, err := os.Open(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening object file: %s\n", err)
		os.Exit(1)
	}
	defer f.Close()

	zr, _ := zlib.NewReader(f)
	data, _ := io.ReadAll(zr)
	zr.Close()

	nullIndex := bytes.IndexByte(data, 0)
	header := string(data[:nullIndex])
	content := data[nullIndex+1:]

	parts := strings.Split(header, " ")
	return parts[0], content
}

func cmdInit() {
	for _, dir := range []string{".git", ".git/objects", ".git/refs"} {
		os.MkdirAll(dir, 0755)
	}
	headFileContents := []byte("ref: refs/heads/main\n")
	os.WriteFile(".git/HEAD", headFileContents, 0644)
	fmt.Println("Initialized git directory")
}

func cmdHashObject(args []string) {
	if len(args) < 2 || args[0] != "-w" {
		fmt.Fprintf(os.Stderr, "usage: mygit hash-object -w <file>\n")
		os.Exit(1)
	}
	filename := args[1]
	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading file: %s\n", err)
		os.Exit(1)
	}
	hash := writeObject("blob", content)
	fmt.Println(hash)
}

func cmdCatFile(args []string) {
	if len(args) < 2 || args[0] != "-p" {
		fmt.Fprintf(os.Stderr, "usage: mygit cat-file -p <hash>\n")
		os.Exit(1)
	}
	hash := args[1]
	objType, content := readObject(hash)
	if objType != "blob" {
		fmt.Fprintf(os.Stderr, "Not a blob object\n")
		os.Exit(1)
	}
	fmt.Print(string(content))
}

func cmdWriteTree() {
	entries := []byte{}
	filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if strings.HasPrefix(path, ".git") {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		content, _ := os.ReadFile(path)
		blobHash := writeObject("blob", content)

		mode := "100644"
		entry := fmt.Sprintf("%s %s\u0000", mode, path)
		entryBytes := []byte(entry)

		hashBytes, _ := hex.DecodeString(blobHash)
		entries = append(entries, entryBytes...)
		entries = append(entries, hashBytes...)
		return nil
	})
	treeHash := writeObject("tree", entries)
	fmt.Println(treeHash)
}

func cmdLsTree(args []string) {
	if len(args) < 2 || args[0] != "--name-only" {
		fmt.Fprintf(os.Stderr, "usage: mygit ls-tree --name-only <tree_hash>\n")
		os.Exit(1)
	}
	treeHash := args[1]
	objType, content := readObject(treeHash)
	if objType != "tree" {
		fmt.Fprintf(os.Stderr, "Not a tree object\n")
		os.Exit(1)
	}

	i := 0
	for i < len(content) {
		nullIndex := bytes.IndexByte(content[i:], 0)
		entry := content[i : i+nullIndex]
		parts := strings.SplitN(string(entry), " ", 2)
		name := parts[1]

		i += nullIndex + 1
		i += 20 // skip SHA bytes

		fmt.Println(name)
	}
}

func cmdCommitTree(args []string) {
	if len(args) < 3 || args[1] != "-m" {
		fmt.Fprintf(os.Stderr, "usage: mygit commit-tree <tree_hash> -m <msg>\n")
		os.Exit(1)
	}

	treeHash := args[0]
	message := strings.Join(args[2:], " ") // join all remaining args as commit message

	content := fmt.Sprintf("tree %s\n\n%s\n", treeHash, message)
	commitHash := writeObject("commit", []byte(content))
	fmt.Println(commitHash)
}

func cmdClone(args []string) {
	fmt.Println("Cloning repository from:", args[0])
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: mygit <command> [<args>...]\n")
		os.Exit(1)
	}
	command := os.Args[1]

	switch command {
	case "init":
		cmdInit()
	case "hash-object":
		cmdHashObject(os.Args[2:])
	case "cat-file":
		cmdCatFile(os.Args[2:])
	case "write-tree":
		cmdWriteTree()
	case "ls-tree":
		cmdLsTree(os.Args[2:])
	case "commit-tree":
		cmdCommitTree(os.Args[2:])
	case "clone":
		cmdClone(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %s\n", command)
		os.Exit(1)
	}
}
