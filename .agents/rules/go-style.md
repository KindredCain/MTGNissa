# Go code style

- Separate imports into standard-library, third-party, and `mtgnissa` project-local groups, in that order, with one blank line between groups. Omit groups that are not used.
- Do not write named functions, methods, or function literals with their body on a single line. Put the body on following lines even when it contains only one statement or is intentionally empty.
- Add a complete GoDoc comment immediately before every exported type. Start the comment with the exact type name and describe the type's responsibility or domain meaning; do not use a placeholder that merely restates the declaration.
