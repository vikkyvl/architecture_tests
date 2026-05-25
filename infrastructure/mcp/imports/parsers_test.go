package imports

import (
	"context"
	"strings"
	"testing"
	"time"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/archguard/project/infrastructure/detector"
)

func runExtractor(t *testing.T, lang, source string) []string {
	t.Helper()
	grammar, ok := detector.LanguageGrammars[lang]
	if !ok {
		grammar = detector.ExtraExtensionGrammars[lang]
	}
	if grammar == nil {
		t.Fatalf("no grammar registered for %q", lang)
	}
	extractor, ok := Extractors[lang]
	if !ok {
		t.Fatalf("no extractor registered for %q", lang)
	}
	parser := sitter.NewParser()
	parser.SetLanguage(grammar)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tree, err := parser.ParseCtx(ctx, nil, []byte(source))
	if err != nil {
		t.Fatalf("parse failed for %s: %v", lang, err)
	}
	defer tree.Close()
	return extractor(tree.RootNode(), []byte(source))
}

func assertContainsAll(t *testing.T, lang string, got []string, want ...string) {
	t.Helper()
	for _, w := range want {
		found := false
		for _, g := range got {
			if g == w || strings.Contains(g, w) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: expected dependency %q in %v", lang, w, got)
		}
	}
}

func TestExtractImportsPHP(t *testing.T) {
	src := `<?php
namespace App\Domain;

use App\Foo\Bar;
use App\Foo\{A, B};
use App\Foo as F;

class Service {}
`
	got := runExtractor(t, "php", src)
	assertContainsAll(t, "php", got, "App\\Foo\\Bar", "App\\Foo")
}

func TestExtractImportsPython(t *testing.T) {
	src := `from app.foo import Bar
import app.foo
import os as system
`
	got := runExtractor(t, "python", src)
	assertContainsAll(t, "python", got, "app.foo.Bar", "app.foo", "os")
}

func TestExtractImportsTypeScript(t *testing.T) {
	src := `import { X } from "./foo";
import type { Y } from "./bar";
import Z from "./baz";
`
	got := runExtractor(t, "typescript", src)
	assertContainsAll(t, "typescript", got, "./foo", "./bar", "./baz")
}

func TestExtractImportsJavaScript(t *testing.T) {
	src := `import X from "./foo";
const Y = require("./bar");
`
	got := runExtractor(t, "javascript", src)
	assertContainsAll(t, "javascript", got, "./foo", "./bar")
}

func TestExtractImportsJava(t *testing.T) {
	src := `package com.example;
import com.example.Foo;
import static com.example.Foo.BAR;
import java.util.*;
`
	got := runExtractor(t, "java", src)
	assertContainsAll(t, "java", got, "com.example.Foo", "java.util")
}

func TestExtractImportsCSharp(t *testing.T) {
	src := `using System.Collections;
using Sys = System;
namespace App {}
`
	got := runExtractor(t, "csharp", src)
	assertContainsAll(t, "csharp", got, "System.Collections", "System")
}

func TestExtractImportsGo(t *testing.T) {
	src := `package main

import "fmt"
import (
	"os"
	"strings"
)
`
	got := runExtractor(t, "go", src)
	assertContainsAll(t, "go", got, "fmt", "os", "strings")
}

func TestExtractImportsRuby(t *testing.T) {
	src := `require "active_record"
require_relative "../foo"
load "x.rb"
`
	got := runExtractor(t, "ruby", src)
	assertContainsAll(t, "ruby", got, "active_record", "../foo", "x.rb")
}

func TestExtractImportsKotlin(t *testing.T) {
	src := `package com.example

import com.example.Foo
import com.example.*
import com.example.Bar as B
`
	got := runExtractor(t, "kotlin", src)
	assertContainsAll(t, "kotlin", got, "com.example.Foo", "com.example.*", "com.example.Bar")
}

func TestExtractImportsSwift(t *testing.T) {
	src := `import Foundation
@testable import AppCore
import struct Foundation.Date
`
	got := runExtractor(t, "swift", src)
	assertContainsAll(t, "swift", got, "Foundation", "AppCore")
}

func TestExtractImportsRust(t *testing.T) {
	src := `use std::collections::HashMap;
use foo::{a, b};
use foo::bar::*;
extern crate serde;
`
	got := runExtractor(t, "rust", src)
	assertContainsAll(t, "rust", got, "std::collections::HashMap", "foo::a", "foo::b", "serde")
}

func TestExtractImportsC(t *testing.T) {
	src := `#include <stdio.h>
#include "config.h"
`
	got := runExtractor(t, "c", src)
	assertContainsAll(t, "c", got, "stdio.h", "config.h")
}

func TestExtractImportsCPP(t *testing.T) {
	src := `#include <vector>
#include "app.hpp"
using namespace std;
`
	got := runExtractor(t, "cpp", src)
	assertContainsAll(t, "cpp", got, "vector", "app.hpp")
}

func TestExtractImportsScala(t *testing.T) {
	src := `package com.example

import scala.collection.mutable.Map
import scala.collection.mutable.{Map, Set}
`
	got := runExtractor(t, "scala", src)
	assertContainsAll(t, "scala", got, "scala.collection.mutable.Map", "scala.collection.mutable.Set")
}

func TestExtractImportsLua(t *testing.T) {
	src := `local foo = require("foo.bar")
local baz = require "baz"
`
	got := runExtractor(t, "lua", src)
	assertContainsAll(t, "lua", got, "foo.bar", "baz")
}

func TestExtractImportsElixir(t *testing.T) {
	src := `defmodule App.Foo do
  alias Foo.Bar
  import Foo
  use Foo.Mixin
end
`
	got := runExtractor(t, "elixir", src)
	assertContainsAll(t, "elixir", got, "Foo.Bar", "Foo")
}

func TestExtractImportsHCL(t *testing.T) {
	src := `module "x" {
  source = "./modules/y"
}

module "z" {
  source = "git::https://example.com/m.git"
}
`
	got := runExtractor(t, "hcl", src)
	assertContainsAll(t, "hcl", got, "./modules/y", "git::https://example.com/m.git")
}
