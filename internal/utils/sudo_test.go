package utils

import (
	"testing"
	"time"
)

func TestNewSudoSession(t *testing.T) {
	timeout := 5 * time.Minute
	session := NewSudoSession(timeout)

	if session == nil {
		t.Error("NewSudoSession() returned nil")
	}

	if session.timeout != timeout {
		t.Errorf("NewSudoSession() timeout = %v, want %v", session.timeout, timeout)
	}
}

func TestSudoSession_RunWithPrivileges(t *testing.T) {
	session := NewSudoSession(5 * time.Minute)

	// 测试无效命令
	if err := session.RunWithPrivileges("invalid_command"); err == nil {
		t.Error("RunWithPrivileges() with invalid command should return error")
	}

	// 测试 echo 命令（不需要 sudo 权限）
	if err := session.RunWithPrivileges("echo", "test"); err != nil {
		t.Errorf("RunWithPrivileges() with echo command failed: %v", err)
	}
}

func TestSudoSession_IsExpired(t *testing.T) {
	timeout := 1 * time.Second
	session := &SudoSession{
		timeout: timeout,
		lastUse: time.Now(),
	}

	// 新会话不应该过期
	if session.IsExpired() {
		t.Error("New session should not be expired")
	}

	// 等待超时
	time.Sleep(timeout + 100*time.Millisecond)

	// 会话应该已过期
	if !session.IsExpired() {
		t.Error("Session should be expired")
	}
}
