package main

import "slices"

var browsers = []string{"chromium", "firefox", "webkit"}

type actionSpec struct {
	required []string
	optional []string
	short    string // 文字列省略形の割当先。空なら省略形なし
}

func (a actionSpec) allows(name string) bool {
	return contains(a.required, name) || contains(a.optional, name)
}

var actionSpecs = map[string]actionSpec{
	"goto":       {required: []string{"url"}, short: "url"},
	"click":      {required: []string{"selector"}, short: "selector"},
	"fill":       {required: []string{"selector", "value"}},
	"type":       {required: []string{"selector", "value"}},
	"press":      {required: []string{"selector", "key"}},
	"select":     {required: []string{"selector", "value"}},
	"hover":      {required: []string{"selector"}, short: "selector"},
	"check":      {required: []string{"selector"}, short: "selector"},
	"uncheck":    {required: []string{"selector"}, short: "selector"},
	"wait":       {required: []string{"duration"}, short: "duration"},
	"waitFor":    {required: []string{"selector"}, short: "selector"},
	"screenshot": {required: []string{"path"}, optional: []string{"full_page"}, short: "path"},
	"pdf":        {required: []string{"path"}, short: "path"},
	"evaluate":   {required: []string{"name", "script"}},
	"extract":    {required: []string{"name", "selector", "attr"}},
	"assert":     {},
}

type assertSpec struct {
	scalar string   // 引数が文字列1つのとき、その格納先
	fields []string // 引数がオブジェクトのときの必須キー
}

var assertSpecs = map[string]assertSpec{
	"url_contains":  {scalar: "value"},
	"url_matches":   {scalar: "value"},
	"visible":       {scalar: "selector"},
	"hidden":        {scalar: "selector"},
	"text_contains": {fields: []string{"selector", "value"}},
	"equals":        {fields: []string{"left", "right"}},
	"not_equals":    {fields: []string{"left", "right"}},
}

func contains(list []string, s string) bool {
	return slices.Contains(list, s)
}
