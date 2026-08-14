# Generates:
# REGEXP_EXTRACT(Book.name, r'^(shelves/[^/]+)')
# extracting "shelves/123" from "shelves/123/books/456".
output "parent_ddl" {
  value = provider::alis::resource_name_ancestor_ddl("Book.name", "shelves")
}

# Nested collections extend the prefix. Generates:
# REGEXP_EXTRACT(Book.name, r'^(shelves/[^/]+/books/[^/]+)')
output "book_ancestor_ddl" {
  value = provider::alis::resource_name_ancestor_ddl("Book.name", "shelves", "books")
}
