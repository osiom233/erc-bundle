package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var (
	key                = []byte{0xd4, 0x1f, 0xdb, 0xe3, 0x37, 0xd0, 0x01, 0x68, 0x0c, 0x2a, 0x4d, 0x43, 0xaf, 0xe5, 0x70, 0xc7, 0x1f, 0xde, 0x85, 0xd8, 0xf3, 0xd4, 0xc4, 0x6f, 0x37, 0x99, 0xc1, 0x8f, 0x1f, 0x50, 0x82, 0x77, 0xac, 0xa7, 0xab, 0x63, 0x32, 0x83, 0x71, 0x0c, 0x2b, 0xb4, 0x1a, 0x07, 0x8e, 0xfb, 0xe7, 0xc1, 0x9c, 0xf0, 0x87, 0xa7, 0xe1, 0x37, 0x75, 0x2a, 0xb7, 0x58, 0x1c, 0x8d, 0x9c, 0x0e, 0x3d, 0xe9}
	pathToDetailsFiles = []string{"songs/unlocks", "songs/packlist", "songs/songlist"}
)

type FileDict struct {
	Path                    string `json:"path"`
	ByteOffset              int    `json:"byteOffset"`
	Length                  int    `json:"length"`
	Sha256HashBase64Encoded string `json:"sha256HashBase64Encoded"`
}

type Metadata struct {
	ApplicationVersionNumber interface{}       `json:"applicationVersionNumber"`
	VersionNumber            interface{}       `json:"versionNumber"`
	PreviousVersionNumber    interface{}       `json:"previousVersionNumber"`
	UUID                     string            `json:"uuid"`
	Removed                  []string          `json:"removed"`
	Added                    []FileDict        `json:"added"`
	PathToHash               map[string]string `json:"pathToHash"`
	PathToDetails            map[string]string `json:"pathToDetails"`
}

func computeSha256(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

func computeSha256Base64(data []byte) string {
	return base64.StdEncoding.EncodeToString(computeSha256(data))
}

func computeHmacSha256Base64(data []byte) string {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func readFileBytes(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func buildFileEntry(filePath, relPath string, offset int) ([]byte, FileDict, string, error) {
	data, err := readFileBytes(filePath)
	if err != nil {
		return nil, FileDict{}, "", err
	}
	hashBase64 := computeSha256Base64(data)
	entry := FileDict{
		Path:                    relPath,
		ByteOffset:              offset,
		Length:                  len(data),
		Sha256HashBase64Encoded: hashBase64,
	}
	return data, entry, hashBase64, nil
}

func getPathToDetails(inputDir string) map[string]string {
	results := make(map[string]string)
	for _, relativePath := range pathToDetailsFiles {
		fullPath := filepath.Join(inputDir, relativePath)
		if data, err := readFileBytes(fullPath); err == nil {
			results[relativePath] = computeHmacSha256Base64(data)
		}
	}
	return results
}

func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:9]
}

func main() {
	inputDir := "./assets/"
	outputFile := "erc.cb"
	outputMetadataFile := "erc.json"

	var (
		appVersion        interface{} = nil
		bundleVersion     interface{} = nil
		prevBundleVersion interface{} = nil
	)

	uuid := generateUUID()
	added := []FileDict{}
	removed := []string{}
	pathToHash := make(map[string]string)
	offset := 0

	output, err := os.Create(outputFile)
	if err != nil {
		log.Fatalf("无法创建输出文件: %v", err)
	}
	defer func(output *os.File) {
		err := output.Close()
		if err != nil {

		}
	}(output)

	err = filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			relPath, _ := filepath.Rel(inputDir, path)
			relPath = strings.ReplaceAll(relPath, string(os.PathSeparator), "/")
			data, entry, hashBase64, err := buildFileEntry(path, relPath, offset)
			if err != nil {
				return err
			}
			added = append(added, entry)
			pathToHash[relPath] = hashBase64
			if _, err := output.Write(data); err != nil {
				return err
			}
			offset += len(data)
		}
		return nil
	})
	if err != nil {
		log.Fatalf("遍历目录时出错: %v", err)
	}

	metadata := Metadata{
		ApplicationVersionNumber: appVersion,
		VersionNumber:            bundleVersion,
		PreviousVersionNumber:    prevBundleVersion,
		UUID:                     uuid,
		Removed:                  removed,
		Added:                    added,
		PathToHash:               pathToHash,
		PathToDetails:            getPathToDetails(inputDir),
	}

	jsonData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		log.Fatalf("序列化 JSON 时出错: %v", err)
	}
	if err := writeFile(outputMetadataFile, jsonData); err != nil {
		log.Fatalf("写入 JSON 文件时出错: %v", err)
	}
}
