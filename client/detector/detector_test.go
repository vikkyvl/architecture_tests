package detector

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	phpHelloFile = `<?php
namespace App\Domain;

class Greeting {
    public function hello(): string {
        return 'hello';
    }
}
`
	pythonHelloFile = `from typing import List


def greet(name: str) -> str:
    return f"hello {name}"


class Greeter:
    def __init__(self, names: List[str]):
        self.names = names

    def all(self) -> List[str]:
        return [greet(n) for n in self.names]
`
	expectedSampleLang = "php"
	minPHPConfidence   = 0.9
)

func TestDetectSampleProjectIsPHP(t *testing.T) {
	root, err := filepath.Abs("../../testdata/sample-project")
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	if _, err := os.Stat(root); err != nil {
		t.Skipf("sample project not present: %v", err)
	}

	r := Detect(root)

	if r.PrimaryLanguage != expectedSampleLang {
		t.Fatalf("expected primary %q, got %q (counts=%v)", expectedSampleLang, r.PrimaryLanguage, r.FileCounts)
	}
	if conf := r.Confidence[expectedSampleLang]; conf < minPHPConfidence {
		t.Fatalf("expected PHP confidence >= %.2f, got %.4f", minPHPConfidence, conf)
	}
}

func TestDetectMixedProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Greeting.php"), phpHelloFile)
	writeFile(t, filepath.Join(dir, "Service.php"), phpHelloFile)
	writeFile(t, filepath.Join(dir, "greeter.py"), pythonHelloFile)

	r := Detect(dir)

	if r.PrimaryLanguage != "php" {
		t.Fatalf("expected php to win on count, got %q (counts=%v)", r.PrimaryLanguage, r.FileCounts)
	}
	if r.FileCounts["php"] != 2 {
		t.Fatalf("expected 2 php files, got %d", r.FileCounts["php"])
	}
	if r.FileCounts["python"] != 1 {
		t.Fatalf("expected 1 python file, got %d", r.FileCounts["python"])
	}
}

func TestDetectMislabelledExtensionFollowsContent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "service.php"), pythonHelloFile)

	r := Detect(dir)

	if r.FileCounts["python"] != 1 {
		t.Fatalf("expected mislabelled .php (Python content) to attribute as python; got counts=%v", r.FileCounts)
	}
	if r.FileCounts["php"] != 0 {
		t.Fatalf("expected zero php attributions; got %d", r.FileCounts["php"])
	}
}

const (
	rubyHelloFile = `require "active_record"
require_relative "../foo"

class Greeting
  def initialize(name)
    @name = name
  end

  def hello
    "hello #{@name}"
  end
end
`
	rustHelloFile = `use std::collections::HashMap;

pub struct Greeting {
    name: String,
}

impl Greeting {
    pub fn new(name: String) -> Self {
        Greeting { name }
    }

    pub fn hello(&self) -> String {
        format!("hello {}", self.name)
    }
}
`
	kotlinHelloFile = `package com.example.greeting

import java.util.List

class Greeting(private val names: List<String>) {
    fun all(): List<String> {
        return names.map { greet(it) }
    }

    private fun greet(name: String): String = "hello $name"
}
`
)

func TestDetectMixedNewLanguages(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Greeting.rb"), rubyHelloFile)
	writeFile(t, filepath.Join(dir, "Greeting.rs"), rustHelloFile)
	writeFile(t, filepath.Join(dir, "Greeting.kt"), kotlinHelloFile)
	writeFile(t, filepath.Join(dir, "Backup.rs"), rustHelloFile)

	r := Detect(dir)

	if r.FileCounts["ruby"] != 1 {
		t.Errorf("expected 1 ruby file, got %d (counts=%v)", r.FileCounts["ruby"], r.FileCounts)
	}
	if r.FileCounts["rust"] != 2 {
		t.Errorf("expected 2 rust files, got %d (counts=%v)", r.FileCounts["rust"], r.FileCounts)
	}
	if r.FileCounts["kotlin"] != 1 {
		t.Errorf("expected 1 kotlin file, got %d (counts=%v)", r.FileCounts["kotlin"], r.FileCounts)
	}
	if r.PrimaryLanguage != "rust" {
		t.Errorf("expected primary=rust (2 files), got %q", r.PrimaryLanguage)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
