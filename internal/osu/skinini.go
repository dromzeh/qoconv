// Package osu builds and serializes osu!mania skins: the skin.ini text and the
// flat output folder / .osk archive.
package osu

import (
	"fmt"
	"strconv"
	"strings"
)

// KV is an ordered collection of key/value pairs for a skin.ini section.
type KV struct {
	keys []string
	m    map[string]string
}

// NewKV returns an empty ordered key/value set.
func NewKV() *KV { return &KV{m: map[string]string{}} }

// Set inserts or updates key, preserving first-insertion order.
func (k *KV) Set(key, val string) {
	if _, ok := k.m[key]; !ok {
		k.keys = append(k.keys, key)
	}
	k.m[key] = val
}

// SetInt is Set with an integer value.
func (k *KV) SetInt(key string, v int) { k.Set(key, strconv.Itoa(v)) }

// Get returns the value for key and whether it was set.
func (k *KV) Get(key string) (string, bool) { v, ok := k.m[key]; return v, ok }

// Len returns the number of keys.
func (k *KV) Len() int { return len(k.keys) }

// Mania is one [Mania] section for a specific keycount.
type Mania struct {
	Keys int
	KV   *KV
}

// NewMania returns a [Mania] block for keys columns.
func NewMania(keys int) *Mania { return &Mania{Keys: keys, KV: NewKV()} }

// Skin is an in-memory osu!mania skin.ini.
type Skin struct {
	General *KV
	Fonts   *KV
	Mania   []*Mania
}

// NewSkin returns an empty skin with initialized [General] and [Fonts].
func NewSkin() *Skin { return &Skin{General: NewKV(), Fonts: NewKV()} }

// Serialize renders the skin.ini using osu!'s `Key: Value` syntax. osu! supports
// multiple [Mania] blocks (one per keycount), which is why this is hand-rolled.
func (s *Skin) Serialize() string {
	var b strings.Builder
	writeSection(&b, "General", s.General)
	if s.Fonts.Len() > 0 {
		b.WriteByte('\n')
		writeSection(&b, "Fonts", s.Fonts)
	}
	for _, m := range s.Mania {
		b.WriteString("\n[Mania]\n")
		fmt.Fprintf(&b, "Keys: %d\n", m.Keys)
		writeKV(&b, m.KV)
	}
	return b.String()
}

func writeSection(b *strings.Builder, name string, kv *KV) {
	fmt.Fprintf(b, "[%s]\n", name)
	writeKV(b, kv)
}

func writeKV(b *strings.Builder, kv *KV) {
	for _, key := range kv.keys {
		fmt.Fprintf(b, "%s: %s\n", key, kv.m[key])
	}
}
