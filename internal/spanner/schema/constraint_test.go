package schema

import "testing"

func TestSpannerTableForeignKeyConstraintDdl(t *testing.T) {
	t.Run("CreateDdl without action", func(t *testing.T) {
		c := &SpannerTableForeignKeyConstraint{
			Name:             "FK_orders_user",
			Column:           "user_id",
			ReferencedTable:  "users",
			ReferencedColumn: "id",
		}
		got, err := c.CreateDdl("orders")
		want := "ALTER TABLE `orders` ADD CONSTRAINT `FK_orders_user` FOREIGN KEY (`user_id`) REFERENCES users(`id`)"
		if err != nil || got != want {
			t.Errorf("CreateDdl() = (%q, %v), want %q", got, err, want)
		}
	})

	t.Run("CreateDdl with ON DELETE CASCADE", func(t *testing.T) {
		c := &SpannerTableForeignKeyConstraint{
			Name:             "FK_orders_user",
			Column:           "user_id",
			ReferencedTable:  "users",
			ReferencedColumn: "id",
			OnDelete:         SpannerTableConstraintActionCascade,
		}
		got, err := c.CreateDdl("orders")
		want := "ALTER TABLE `orders` ADD CONSTRAINT `FK_orders_user` FOREIGN KEY (`user_id`) REFERENCES users(`id`) ON DELETE CASCADE"
		if err != nil || got != want {
			t.Errorf("CreateDdl() = (%q, %v), want %q", got, err, want)
		}
	})

	t.Run("DropForeignKeyConstraintDdl", func(t *testing.T) {
		if got := DropForeignKeyConstraintDdl("orders", "FK_orders_user"); got != "ALTER TABLE `orders` DROP CONSTRAINT `FK_orders_user`" {
			t.Errorf("DropForeignKeyConstraintDdl() = %q", got)
		}
	})

	t.Run("missing fields error", func(t *testing.T) {
		cases := []*SpannerTableForeignKeyConstraint{
			{Column: "c", ReferencedTable: "t", ReferencedColumn: "r"}, // no name
			{Name: "n", ReferencedTable: "t", ReferencedColumn: "r"},  // no column
			{Name: "n", Column: "c", ReferencedColumn: "r"},           // no referenced table
			{Name: "n", Column: "c", ReferencedTable: "t"},            // no referenced column
		}
		for i, c := range cases {
			if _, err := c.CreateDdl("orders"); err == nil {
				t.Errorf("case %d: expected error for incomplete constraint", i)
			}
		}
		if _, err := (&SpannerTableForeignKeyConstraint{Name: "n", Column: "c", ReferencedTable: "t", ReferencedColumn: "r"}).CreateDdl(""); err == nil {
			t.Error("expected error for empty table")
		}
	})

	t.Run("nil receiver returns empty", func(t *testing.T) {
		var c *SpannerTableForeignKeyConstraint
		got, err := c.CreateDdl("orders")
		if got != "" || err != nil {
			t.Errorf("nil receiver = (%q, %v), want empty", got, err)
		}
	})
}
