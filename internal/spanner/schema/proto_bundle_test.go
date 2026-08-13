package schema

import "testing"

func TestProtoBundleDdl(t *testing.T) {
	names := []string{"`com.example.Msg`", "`com.example.Msg.Nested`"}

	t.Run("CreateProtoBundleDdl", func(t *testing.T) {
		if got := CreateProtoBundleDdl(names); got != "CREATE PROTO BUNDLE (`com.example.Msg`, `com.example.Msg.Nested`)" {
			t.Errorf("CreateProtoBundleDdl() = %q", got)
		}
	})

	t.Run("AlterProtoBundleInsertDdl", func(t *testing.T) {
		if got := AlterProtoBundleInsertDdl(names); got != "ALTER PROTO BUNDLE INSERT (`com.example.Msg`, `com.example.Msg.Nested`)" {
			t.Errorf("AlterProtoBundleInsertDdl() = %q", got)
		}
	})

	t.Run("AlterProtoBundleUpdateDdl", func(t *testing.T) {
		if got := AlterProtoBundleUpdateDdl(names); got != "ALTER PROTO BUNDLE UPDATE (`com.example.Msg`, `com.example.Msg.Nested`)" {
			t.Errorf("AlterProtoBundleUpdateDdl() = %q", got)
		}
	})
}
