package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
)

func doReadCache(path string, data interface{}) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		} else {
			return err
		}
	}

	b, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	err = json.Unmarshal(b, data)
	if err != nil {
		return err
	}

	return nil
}

func readCache(path string, data interface{}) error {
	err := doReadCache(path, data)
	if err != nil {
		log.Printf("Couldn't read cache from %s: %s", path, err)
	}
	return err
}

func doWriteCache(path string, data interface{}) error {
	f, err := os.CreateTemp("", "sls")
	if err != nil {
		return err
	}
	tmpFile := f.Name()

	b, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = f.Write(b)
	if err != nil {
		return err
	}

	err = f.Close()
	if err != nil {
		return err
	}

	err = os.Rename(tmpFile, path)
	if err != nil {
		return err
	}

	return nil
}

func writeCache(path string, data interface{}) error {
	err := doWriteCache(path, data)
	if err != nil {
		log.Printf("Couldn't write cache to %s: %s", path, err)
	}
	return err
}
