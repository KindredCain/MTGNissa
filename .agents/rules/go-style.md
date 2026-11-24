# Go code style

- Use UpperCamelCase for exported names and conventional casing for common initialisms.
- Separate imports into standard-library, third-party, and `mtgnissa` project-local groups, in that order, with one blank line between groups. Omit groups that are not used.
- Place every file-level type declaration before all function and method declarations in the same file.
- Do not write named functions, methods, or function literals with their body on a single line. Put the body on following lines even when it contains only one statement or is intentionally empty.
- Add a complete GoDoc comment immediately before every exported type, function, and method. Start the comment with the exact declared name and describe its responsibility or domain meaning; do not use a placeholder that merely restates the declaration.
- Explain the reasons, assumptions, or limits behind non-obvious logic rather than narrating the code.
- Format changed Go files with `gofmt`.
