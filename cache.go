package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
)

type cache struct {
	fileStore *os.File
}

func NewCache(cachePath string) *cache {
	// load cache file on each run
	f, err := os.OpenFile(cachePath, os.O_RDWR|os.O_CREATE, 0775)
	if err != nil {
		log.Fatalf("could not open cache file: %v\n", err)
	}

	return &cache{f}
}

func (c *cache) loadCache() map[int]struct{} {
	result := make(map[int]struct{})
	scanner := bufio.NewScanner(c.fileStore)
	for scanner.Scan() {
		i, err := strconv.Atoi(scanner.Text())
		if err != nil {
			log.Printf("Skipping cache entry: %v", err)

			continue
		}

		result[i] = struct{}{}
	}

	fmt.Printf("loading cache: %d lines\n", len(result))

	return result
}

func (c *cache) writeCache(cachePipelineIDs map[int]struct{}) error {
	err := c.fileStore.Truncate(0)
	if err != nil {
		return fmt.Errorf("error truncating cache: %v", err)
	}
	_, err = c.fileStore.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("error seeking cache: %v", err)
	}

	w := bufio.NewWriter(c.fileStore)
	defer w.Flush()

	for k := range cachePipelineIDs {
		_, err := w.WriteString(fmt.Sprintf("%d\n", k))
		if err != nil {
			return fmt.Errorf("error writing cache: %v", err)
		}
	}

	return nil
}
