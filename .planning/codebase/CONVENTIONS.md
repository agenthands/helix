# Coding Conventions

**Analysis Date:** 2026-04-07

## Naming Patterns

**Files:**
- snake_case for all Python files: `file_tools.py`, `symbol_tools.py`, `serena_config.py`
- Test files follow pattern: `test_<domain>.py` (e.g., `test_python_basic.py`, `test_serena_agent.py`)
- Package directories use snake_case: `src/serena/tools/`, `src/solidlsp/language_servers/`

**Classes:**
- PascalCase for all class names: `ReadFileTool`, `LanguageServerManager`, `SerenaConfig`
- Tool classes end with `Tool`: `ReadFileTool`, `CreateTextFileTool`, `ReplaceContentTool`
- Enum classes inherit from StrEnum: `class LineType(StrEnum):`
- Marker classes for tools: `ToolMarkerCanEdit`, `ToolMarkerSymbolicRead`, `ToolMarkerBeta`

**Functions/Methods:**
- snake_case for all function and method names: `apply()`, `get_symbol_overview()`, `create_language_server_manager()`
- Private/internal methods start with underscore: `_create_ls()`, `_limit_length()`
- Context managers prefixed with `start_` or suffixed with `_context`: `start_ls_context()`, `project_context()`

**Variables:**
- snake_case for variables: `repo_path`, `original_content`, `language_server`
- Constants in UPPER_SNAKE_CASE: `DEFAULT_SOURCE_FILE_ENCODING`, `SERENA_MANAGED_DIR_NAME`
- Type variables use single uppercase letter: `T`, `TTool` (with `TypeVar` bounds where needed)
- Private instance variables prefixed with underscore: `self._tool_names`, `self._encoding`
- Boolean variables often prefixed: `is_ci`, `is_windows`, `has_malformed_name`

**Module-level patterns:**
- Each module has a module logger: `log = logging.getLogger(__name__)`
- Docstring at module top level: `"""File and file system-related tools..."""`

## Code Style

**Formatting:**
- Tool: ruff (run via `uv run poe format`)
- Line length: 140 characters (configured in `pyproject.toml`)
- Quote style: double quotes for strings
- Indent: 4 spaces
- Docstring formatting: enabled via `docstring-code-format = true`

**Linting:**
- Tool: ruff check (run via `uv run poe lint`)
- Configuration: `pyproject.toml` with extensive rule selection
- Key ignored rules: unused variables allowed, long lines allowed, unspecific except clauses allowed for error recovery
- Max complexity: 20 (McCabe)

**Type Checking:**
- Tool: mypy (run via `uv run poe type-check`)
- Settings: strict mode with `disallow_untyped_defs = true` for core code
- Exception: `disallow_untyped_defs = false` for test code (tests use less strict typing)
- Use `TYPE_CHECKING` guard for circular import prevention: `if TYPE_CHECKING: from serena.agent import SerenaAgent`

## Import Organization

**Order (enforced by ruff):**
1. Standard library: `import os`, `from pathlib import Path`
2. Third-party: `import pytest`, `from pydantic import BaseModel`
3. Local relative: `from serena.tools import Tool`, `from solidlsp.ls_config import Language`
4. Within each group, sort alphabetically

**Path Aliases:**
- No path aliases configured; use absolute imports from `src/` roots
- Import from public namespaces: `from serena.tools import ReadFileTool` not internal paths
- Use `TYPE_CHECKING` guards for optional/circular imports

**Example structure from `src/serena/tools/file_tools.py`:**
```python
import os
from collections import defaultdict
from fnmatch import fnmatch
from pathlib import Path
from typing import Literal

from serena.tools import SUCCESS_RESULT, EditedFileContext, Tool, ToolMarkerCanEdit, ToolMarkerOptional
from serena.util.file_system import scan_directory
from serena.util.text_utils import ContentReplacer, search_files
```

## Error Handling

**Patterns:**
- Explicit exception types preferred over broad `except Exception`
- Language server failures use `SolidLSPException` with checks like `e.is_language_server_terminated()`
- Tool execution wraps errors in `apply_ex()` method with automatic retry on language server restart
- Custom exceptions extend Exception: `class ProjectNotFoundError(Exception): pass`
- Validation exceptions use `ValueError` with descriptive messages
- File path validation: `self.project.validate_relative_path(relative_path, require_not_ignored=True)`
- File not found: `raise FileNotFoundError(f"Relative path {relative_path} does not exist.")`

**Error recovery:**
- Language server crashes trigger automatic restart via `get_language_server_manager_or_raise().restart_language_server()`
- Tool execution catches exceptions and returns error strings to LLM: `return f"Error executing tool: {e.__class__.__name__} - {e}"`
- Graceful degradation: shortened result factories try progressively shorter versions when output too long

## Logging

**Framework:** sensai.util.logging (custom wrapper)

**Patterns:**
- Module-level logger: `log = logging.getLogger(__name__)`
- Log levels: INFO for normal operations, WARNING for non-fatal issues, ERROR for failures
- Use f-strings in log messages: `log.info(f"Starting language server for {language} {repo_path}")`
- Parameter logging in tool execution: `log.info(f"{self.get_name_from_cls()}: {dict_string(params)}")`
- Exception logging: `log.error(f"Error executing tool: {e}", exc_info=e)` includes exception info
- LSP communication tracing: optional via `trace_lsp_communication=True` parameter

**Configuration:**
- Logging configured at test startup: `configure(level=logging.INFO)` in conftest
- Log viewer available via GUI in production mode
- Default log level in tests: ERROR to reduce noise

## Comments

**When to Comment:**
- Add docstrings for all public methods and classes (enforced by ruff D rules)
- Explain non-obvious algorithm choices: `# capture kind names and depth-0 snapshots before grouping, which mutates the dicts`
- Explain temporary workarounds: `# TODO: Fix when language server behavior changes`
- Document parameter constraints in docstrings

**JSDoc/Docstring Style:**
- Use triple-quoted docstrings for modules, classes, and functions
- Include parameter descriptions with type hints: `:param relative_path: the relative path to the file to read`
- Include return type description: `:return: a message indicating success or failure`
- Example from `ReadFileTool.apply()`:
```python
def apply(self, relative_path: str, start_line: int = 0, end_line: int | None = None, max_answer_chars: int = -1) -> str:
    """
    Reads the given file or a chunk of it. Generally, symbolic operations
    like find_symbol or find_referencing_symbols should be preferred if you know which symbols you are looking for.

    :param relative_path: the relative path to the file to read
    :param start_line: the 0-based index of the first line to be retrieved.
    :param end_line: the 0-based index of the last line to be retrieved (inclusive). If None, read until the end of the file.
    :param max_answer_chars: if the file (chunk) is longer than this number of characters,
        no content will be returned. Don't adjust unless there is really no other way to get the content
        required for the task.
    :return: the full text of the file at the given relative path
    """
```

## Function Design

**Size Guidelines:**
- Prefer focused, single-responsibility functions (max complexity 20)
- Break large methods into private helper methods with leading underscore: `_create_ls()`, `_limit_length()`
- Use nested functions for context managers: `start_ls_context()` yields after logging entry

**Parameters:**
- Use type hints on all parameters: `def apply(self, relative_path: str, start_line: int = 0, ...) -> str:`
- Optional parameters with `| None`: `repo_path: str | None = None`
- Use keyword-only arguments in dataclasses: `@dataclass(kw_only=True)`
- Union types use `|` syntax (Python 3.10+): `Literal["read", "write"]`
- Avoid mutable defaults; use `None` with factory pattern: `def __init__(self, items: list[T] | None = None): self.items = items or []`

**Return Values:**
- Single return type clearly typed: `def get_name(self) -> str:`
- Union returns use `|`: `str | None`
- Strings for tool results (consumed by LLM): tools always return `str`
- Context managers use `Iterator[T]` return type: `def start_ls_context(...) -> Iterator[SolidLanguageServer]:`
- Use `SUCCESS_RESULT = "OK"` constant for successful operations

## Module Design

**Exports:**
- Public classes/functions exposed at module level: `from serena.tools import Tool, ReadFileTool`
- Private implementation details use leading underscore: `_create_ls()`, `_disabled_languages`
- No star imports: always use explicit imports

**Barrel Files:**
- `src/serena/tools/__init__.py` exports public tool API
- `src/serena/__init__.py` exports version and main classes
- Each language server directory has `__init__.py` exporting main class

**Module conventions:**
- Tools inherit from `Tool` base class: `class ReadFileTool(Tool):`
- Tools implement `apply()` method with documented parameters
- Tools use marker classes for capabilities: `class CreateTextFileTool(Tool, ToolMarkerCanEdit):`
- Docstring on class and on `apply()` method both required for MCP tool registration

## Dataclass Usage

**Pattern (from `TextLine` in `text_utils.py`):**
```python
@dataclass(kw_only=True)
class TextLine:
    """Represents a line of text with information on how it relates to the match."""
    
    line_number: int
    line_content: str
    match_type: LineType
    """Represents the type of line (match, prefix, postfix)"""

    def get_display_prefix(self) -> str:
        """Get the display prefix for this line based on the match type."""
        if self.match_type == LineType.MATCH:
            return "  >"
        return "..."
```

- Use `@dataclass(kw_only=True)` for required keyword arguments
- Add field docstrings on the same line or after the field
- Include methods for formatting/display
- Use `field(default_factory=list)` for mutable defaults

## Code Organization Examples

**Tool structure (`ReadFileTool`):**
- Class docstring explains purpose
- `apply()` method signature shows all parameters with types
- Docstring on `apply()` explains each parameter and return
- Implementation handles edge cases (start_line, end_line)
- Uses helper methods for output limiting: `self._limit_length(result, max_answer_chars)`

**Test structure (`test_python_basic.py`):**
- Class per logical test group: `class TestPythonLanguageServerBasics:`
- Pytest marker on class: `@pytest.mark.python`
- Parametrized fixtures: `@pytest.mark.parametrize("language_server", PYTHON_BACKEND_LANGUAGES, indirect=True)`
- Clear test names: `test_request_references_user_class()`
- Assertion-heavy verification of expected conditions

---

*Convention analysis: 2026-04-07*
