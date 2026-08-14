# Generates:
# REGEXP_EXTRACT(Book.name, r'books/([^/]+)')
# extracting "456" from "shelves/123/books/456".
output "book_id_ddl" {
  value = provider::alis::resource_name_id_ddl("Book.name", "books")
}
