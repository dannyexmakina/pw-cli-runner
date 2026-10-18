package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mxschmitt/playwright-go"
)

type Diagnosis struct {
	Count    int           `json:"count"`
	Visible  *bool         `json:"visible"`
	Enabled  *bool         `json:"enabled"`
	Editable *bool         `json:"editable"`
	Matched  *Matched      `json:"matched"`
	Resolved []ChainResult `json:"resolved"`
}

type Matched struct {
	Tag   string            `json:"tag"`
	Role  string            `json:"role"`
	Name  string            `json:"name"`
	Text  string            `json:"text"`
	Attrs map[string]string `json:"attrs"`
	Path  string            `json:"path"`
}

type ChainResult struct {
	Selector string `json:"selector"`
	Count    int    `json:"count"`
}

// OK は要素がちょうど1件に一致したか。
func (d *Diagnosis) OK() bool {
	return d != nil && d.Count == 1
}

// Short は stderr 用の1行要約。
func (d *Diagnosis) Short() string {
	if !d.OK() {
		return fmt.Sprintf("count=%d ✗", d.Count)
	}
	s := "count=1 ✓"
	if m := d.Matched; m != nil {
		label, text := m.Role, m.Name
		if label == "" {
			label, text = m.Tag, m.Text
		}
		s += fmt.Sprintf(" → %s %q", label, truncate(text, 30))
	}
	return s
}

// Message は selector 種別の失敗時のエラーメッセージ。
func (d *Diagnosis) Message(sel string) string {
	if d.Count == 0 {
		for _, r := range d.Resolved {
			if r.Count == 0 {
				return fmt.Sprintf(errSelectorNoMatchAt, sel, r.Selector)
			}
		}
		return fmt.Sprintf(errSelectorNoMatch, sel)
	}
	return fmt.Sprintf(errSelectorMulti, sel, d.Count)
}

// splitChain は `>>` 連鎖を引用符の外側でのみ分割する。
func splitChain(sel string) []string {
	var parts []string
	var quote rune
	start := 0
	rs := []rune(sel)
	for i := 0; i < len(rs); i++ {
		c := rs[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'' || c == '`':
			quote = c
		case c == '>' && i+1 < len(rs) && rs[i+1] == '>':
			parts = append(parts, strings.TrimSpace(string(rs[start:i])))
			i++
			start = i + 1
		}
	}
	return append(parts, strings.TrimSpace(string(rs[start:])))
}

const describeJS = `el => {
  const implicit = () => {
    const t = el.tagName.toLowerCase();
    const type = (el.getAttribute('type') || '').toLowerCase();
    if (t === 'button') return 'button';
    if (t === 'a' && el.hasAttribute('href')) return 'link';
    if (t === 'select') return 'combobox';
    if (t === 'textarea') return 'textbox';
    if (t === 'img') return 'img';
    if (/^h[1-6]$/.test(t)) return 'heading';
    if (t === 'input') {
      if (['button', 'submit', 'reset', 'image'].includes(type)) return 'button';
      if (type === 'checkbox') return 'checkbox';
      if (type === 'radio') return 'radio';
      if (type === 'hidden') return '';
      return 'textbox';
    }
    return '';
  };
  const text = (el.innerText || el.textContent || '').trim().replace(/\s+/g, ' ');
  const labelled = (el.getAttribute('aria-labelledby') || '').split(/\s+/)
    .map(id => document.getElementById(id)).filter(Boolean)
    .map(n => n.textContent.trim()).join(' ');
  const labels = el.labels ? [...el.labels].map(l => l.textContent.trim()).join(' ') : '';
  const name = el.getAttribute('aria-label') || labelled || labels || el.getAttribute('alt') ||
    el.getAttribute('title') || el.getAttribute('placeholder') ||
    (['button', 'link', 'heading'].includes(el.getAttribute('role') || implicit()) ? text : '') ||
    (el.tagName === 'INPUT' && ['submit', 'button'].includes(el.type) ? el.value : '');
  const attrs = {};
  for (const a of ['id', 'class', 'name', 'type', 'href', 'value', 'data-testid']) {
    const v = el.getAttribute(a);
    if (v !== null) attrs[a] = v.length > 80 ? v.slice(0, 80) + '…' : v;
  }
  const path = [];
  for (let n = el; n && n.nodeType === 1 && path.length < 6; n = n.parentElement) {
    let p = n.tagName.toLowerCase();
    if (n.id) p += '#' + n.id;
    else if (n.classList.length) p += '.' + [...n.classList].slice(0, 2).join('.');
    path.unshift(p);
  }
  return { tag: el.tagName.toLowerCase(), role: el.getAttribute('role') || implicit(),
    name: name.slice(0, 80), text: text.slice(0, 80), attrs, path: path.join(' > ') };
}`

// diagnose はセレクタの現在の解決状況を調べる。操作は行わない。
func diagnose(page playwright.Page, sel string) *Diagnosis {
	loc := page.Locator(sel)
	d := &Diagnosis{}
	d.Count, _ = loc.Count()

	if parts := splitChain(sel); len(parts) > 1 {
		for i := range parts {
			prefix := strings.Join(parts[:i+1], " >> ")
			n, _ := page.Locator(prefix).Count()
			d.Resolved = append(d.Resolved, ChainResult{Selector: parts[i], Count: n})
			if n == 0 {
				break
			}
		}
	}
	if d.Count != 1 {
		return d
	}

	quick := playwright.LocatorIsVisibleOptions{}
	if v, err := loc.IsVisible(quick); err == nil {
		d.Visible = &v
	}
	if v, err := loc.IsEnabled(playwright.LocatorIsEnabledOptions{Timeout: playwright.Float(1000)}); err == nil {
		d.Enabled = &v
	}
	if v, err := loc.IsEditable(playwright.LocatorIsEditableOptions{Timeout: playwright.Float(1000)}); err == nil {
		d.Editable = &v
	}
	if raw, err := loc.Evaluate(describeJS, nil, playwright.LocatorEvaluateOptions{Timeout: playwright.Float(1000)}); err == nil {
		if b, err := json.Marshal(raw); err == nil {
			var m Matched
			if json.Unmarshal(b, &m) == nil {
				d.Matched = &m
			}
		}
	}
	return d
}

func truncate(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n]) + "…"
}
