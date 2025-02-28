package dca_test

import (
	"github.com/1gm/dca"
	"testing"
)

func TestHasAWSParamStorePrefix(t *testing.T) {
	tt := []struct {
		input    string
		expected bool
	}{
		{"awsssm://foobar", true},
		{"awsssme://foobar", true},
		{"awwssss", false},
	}
	for i, tc := range tt {
		if want, got := tc.expected, dca.HasAWSParamStorePrefix(tc.input); got != tc.expected {
			t.Errorf("%d: want %v got %v", i, want, got)
		}
	}
}

func TestHasAWSParamStorePlaintextPrefix(t *testing.T) {
	tt := []struct {
		input    string
		expected bool
	}{
		{"awsssm://foobar", true},
		{"awsssme://foobar", false},
		{"", false},
	}
	for i, tc := range tt {
		if want, got := tc.expected, dca.HasAWSParamStorePlaintextPrefix(tc.input); got != tc.expected {
			t.Errorf("%d: want %v got %v", i, want, got)
		}
	}
}

func TestHasAWSParamStoreEncryptedPrefix(t *testing.T) {
	tt := []struct {
		input    string
		expected bool
	}{
		{"awsssm://foobar", false},
		{"awsssme://foobar", true},
		{"", false},
	}
	for i, tc := range tt {
		if want, got := tc.expected, dca.HasAWSParamStoreEncryptedPrefix(tc.input); got != tc.expected {
			t.Errorf("%d: want %v got %v", i, want, got)
		}
	}
}

func TestStripAWSParamStorePrefix(t *testing.T) {
	tt := []struct {
		input    string
		expected string
	}{
		{"awsssm://foobar", "foobar"},
		{"awsssme://foobar", "foobar"},
		{"", ""},
	}
	for i, tc := range tt {
		if want, got := tc.expected, dca.StripAWSParamStorePrefix(tc.input); got != tc.expected {
			t.Errorf("%d: want %v got %v", i, want, got)
		}
	}
}
