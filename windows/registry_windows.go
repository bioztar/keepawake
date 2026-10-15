package main

import (
	"errors"

	"golang.org/x/sys/windows/registry"
)

const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

// hkcuKey is a key under HKEY_CURRENT_USER (no admin rights needed).
type hkcuKey string

func (k hkcuKey) Get(name string) (string, bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, string(k), registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	defer key.Close()
	v, _, err := key.GetStringValue(name)
	if errors.Is(err, registry.ErrNotExist) {
		return "", false, nil
	}
	return v, err == nil, err
}

func (k hkcuKey) Set(name, value string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, string(k), registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.SetStringValue(name, value)
}

func (k hkcuKey) Delete(name string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, string(k), registry.SET_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer key.Close()
	if err := key.DeleteValue(name); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}
