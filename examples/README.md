# Manual examples

These templates are safe, fictional starting points for exercising the wizard
from a real terminal. They contain example-only values: never replace them with
real credentials in the repository.

| Template | What it demonstrates |
| --- | --- |
| `basic.env.example` | Prompts, descriptions, required values, placeholders, and closed options |
| `typed-values.env.example` | Explicit string, integer, boolean, port, and URL fields |
| `secrets-and-fixed.env.example` | Masked secrets, required secrets, and template-owned fixed values |
| `merged-sections.env.example` | Implicit and explicit sections, including reopened sections that merge in the wizard |

From the repository root, create an ignored output directory once:

```powershell
New-Item -ItemType Directory -Force .tmp/manual
```

On Linux or macOS, the equivalent command is `mkdir -p .tmp/manual`. Then run an
example with the source checkout:

```text
go run ./cmd/env-wizard --template examples/basic.env.example --output .tmp/manual/basic.env
```

Replace both filenames to try another template. If `env-wizard` is already
installed, use it instead of `go run ./cmd/env-wizard`. Run the same command a
second time to exercise reuse of an existing output, no-op detection, overwrite
confirmation, and timestamped backup creation after changing a value.

The generated files can contain the values entered into secret fields. The
`.tmp` directory is ignored by Git in this repository, but the files are still
plaintext and should be treated accordingly.
