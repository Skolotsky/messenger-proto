//go:build !feedfaults

package service

func fault(string) bool { return false }
