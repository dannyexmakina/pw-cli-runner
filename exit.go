package main

import (
	"errors"
	"fmt"
	"io"
)

// 終了コード（設計書 §9.1）
const (
	exitOK             = 0
	exitScenarioFailed = 1
	exitUsage          = 2
	exitInvalidYAML    = 3
	exitEnv            = 4
	exitInternal       = 5
)

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string {
	if e.err == nil {
		return fmt.Sprintf("exit %d", e.code)
	}
	return e.err.Error()
}

func (e *exitError) Unwrap() error { return e.err }

func exitErr(code int, err error) error {
	return &exitError{code: code, err: err}
}

// exitCode は RunE が返したエラーを終了コードに変換し、理由を w に出す。
// exitError でないものは cobra の引数・フラグ解析エラーとみなす。
func exitCode(err error, w io.Writer) int {
	if err == nil {
		return exitOK
	}
	var e *exitError
	if !errors.As(err, &e) {
		fmt.Fprintln(w, "error:", err)
		return exitUsage
	}
	if e.err != nil {
		fmt.Fprintln(w, "error:", e.err)
	}
	return e.code
}
