Change the signature of the public function `ProcessRequest` to accept a
`context.Context` as its first parameter. Before editing, analyze the blast
radius to understand all callers. Update all callers in the package.
