# Testing Patterns

**Analysis Date:** 2026-04-07

## Test Framework

**Runner:**
- pytest 8.4.1
- Config: `pyproject.toml` with `addopts = "--snapshot-patch-pycharm-diff"`
- Run commands:
  ```bash
  uv run poe test                    # Run all tests (uses PYTEST_MARKERS env var)
  uv run poe test -m "python or go"  # Run specific language tests
  uv run poe test -m vue             # Run Vue tests
  uv run poe test -m snapshot        # Run snapshot tests only
  ```

**Assertion Library:**
- Built-in pytest assertions (`assert`)
- Snapshot testing via syrupy 4.9.1: `assert actual == snapshot`
- Comparison utilities from conftest helpers

**Test Markers:**
Available pytest markers for selective testing:
- Language markers: `python`, `go`, `java`, `rust`, `typescript`, `vue`, `php`, `perl`, `powershell`, `csharp`, `elixir`, `terraform`, `clojure`, `swift`, `bash`, `ruby`, `ruby_solargraph`, `cpp`, `csharp`, `fsharp`, `yaml`, `julia`, `fortran`, `haskell`, `toml`, `matlab`, `systemverilog`, `hlsl`, `lean4`, `solidity`, `ansible`, `kotlin`, `groovy`, `zig`, `lua`, `luau`, `nix`, `dart`, `erlang`, `ocaml`, `scala`, `al`, `rego`, `markdown`, `pascal`, `r`, `elm`
- `snapshot` - for symbolic editing operation tests
- `slow` - tests requiring multiple Expert instances with ~60-90s startup time

## Test File Organization

**Location Pattern:**
- Language-specific tests: `test/solidlsp/<language>/test_<domain>.py`
  - Example: `test/solidlsp/python/test_python_basic.py`
  - Example: `test/solidlsp/typescript/test_typescript_basic.py`
- Serena agent tests: `test/serena/test_<domain>.py`
  - Example: `test/serena/test_serena_agent.py`
  - Example: `test/serena/test_symbol_editing.py`
- Utility tests: `test/serena/util/test_<domain>.py`
  - Example: `test/serena/util/test_file_system.py`

**Naming:**
- Test files: `test_<domain>.py`
- Test classes: `Test<Feature>` (PascalCase): `TestPythonLanguageServerBasics`, `TestProjectBasics`
- Test methods: `test_<specific_scenario>` (snake_case): `test_request_references_user_class`, `test_retrieve_content_around_line`

**Directory structure:**
```
test/
├── conftest.py                    # Global fixtures and utilities
├── resources/
│   ├── repos/                     # Test repositories per language
│   │   ├── python/test_repo/
│   │   ├── typescript/test_repo/
│   │   └── go/test_repo/
│   └── __snapshots__/             # Snapshot test outputs
├── solidlsp/
│   ├── conftest.py                # Language-specific fixture utilities
│   ├── python/
│   │   ├── test_python_basic.py
│   │   ├── test_python_auto_update.py
│   │   └── __init__.py
│   └── typescript/
│       ├── test_typescript_basic.py
│       └── __init__.py
└── serena/
    ├── __snapshots__/             # Snapshot outputs for serena tests
    ├── test_serena_agent.py
    ├── test_symbol_editing.py
    ├── util/
    │   └── test_file_system.py
    └── config/
        └── test_serena_config.py
```

## Test Structure

**Suite Organization (from `test_python_basic.py`):**
```python
import os
import pytest

from serena.project import Project
from serena.util.text_utils import LineType
from solidlsp import SolidLanguageServer
from test.solidlsp.conftest import PYTHON_BACKEND_LANGUAGES, format_symbol_for_assert, has_malformed_name, request_all_symbols

@pytest.mark.python
class TestPythonLanguageServerBasics:
    """Test basic functionality of the language server."""

    @pytest.mark.parametrize("language_server", PYTHON_BACKEND_LANGUAGES, indirect=True)
    def test_request_references_user_class(self, language_server: SolidLanguageServer) -> None:
        """Test request_references on the User class."""
        # Arrange: Get the symbol
        file_path = os.path.join("test_repo", "models.py")
        symbols = language_server.request_document_symbols(file_path).get_all_symbols_and_roots()
        user_symbol = next((s for s in symbols[0] if s.get("name") == "User"), None)
        
        # Act: Request references
        sel_start = user_symbol["selectionRange"]["start"]
        references = language_server.request_references(file_path, sel_start["line"], sel_start["character"])
        
        # Assert: Verify results
        assert len(references) > 1, "User class should be referenced in multiple files"
```

**Patterns:**
- Pytest marker on class: `@pytest.mark.python`
- Parametrized fixtures: `@pytest.mark.parametrize("language_server", PYTHON_BACKEND_LANGUAGES, indirect=True)`
- Docstring on test method explaining what is being tested
- Arrange-Act-Assert pattern (explicit or implicit)
- Comments only where clarity is needed

## Fixtures

**Scope Hierarchy (from `test/conftest.py`):**

**Session-level:**
```python
@pytest.fixture(scope="session")
def resources_dir() -> Path:
    """Path to the test resources directory."""
    current_dir = Path(__file__).parent
    return current_dir / "resources"
```

**Module-level (reused across test module):**
```python
@pytest.fixture(scope="module")
def language_server(request: LanguageParamRequest):
    """Create a language server instance configured for the specified language."""
    if not hasattr(request, "param"):
        raise ValueError("Language parameter must be provided via pytest.mark.parametrize")
    
    language = request.param
    with start_default_ls_context(language) as ls:
        yield ls
```

**Context managers:**
```python
@contextmanager
def start_ls_context(
    language: Language,
    repo_path: str | None = None,
    ignored_paths: list[str] | None = None,
    trace_lsp_communication: bool = False,
    ls_specific_settings: dict[Language, dict[str, Any]] | None = None,
    solidlsp_dir: Path | None = None,
) -> Iterator[SolidLanguageServer]:
    ls = _create_ls(language, repo_path, ignored_paths, trace_lsp_communication, ls_specific_settings, solidlsp_dir)
    log.info(f"Starting language server for {language} {repo_path}")
    ls.start()
    try:
        log.info(f"Language server started for {language} {repo_path}")
        yield ls
    finally:
        log.info(f"Stopping language server for {language} {repo_path}")
        try:
            ls.stop(shutdown_timeout=5)
        except Exception as e:
            log.warning(f"Warning: Error stopping language server: {e}")
            # try to force cleanup
            if hasattr(ls, "server") and hasattr(ls.server, "process"):
                try:
                    ls.server.process.terminate()
                except:
                    pass
```

**Parametrized Fixtures (indirect=True):**
```python
@pytest.fixture(scope="module")
def project(request: LanguageParamRequest, repo_root_override: str | None = None) -> Iterator[Project]:
    """Create a Project for the specified language.

    This fixture requires a language parameter via pytest.mark.parametrize:

    Example:
    ```
    @pytest.mark.parametrize("project", [Language.PYTHON], indirect=True)
    def test_python_project(project: Project) -> None:
        # Use the Python project to test something
        pass
    ```
    """
    if not hasattr(request, "param"):
        raise ValueError("Language parameter must be provided via pytest.mark.parametrize")
    language = request.param
    with project_context(language, repo_root_override) as project:
        yield project
```

**Test fixture from Serena agent tests (from `test/serena/test_serena_agent.py`):**
```python
@pytest.fixture
def serena_config():
    config = SerenaConfig(gui_log_window=False, web_dashboard=False, log_level=logging.ERROR)
    
    # Create test projects for all supported languages
    test_projects = []
    for language in [Language.PYTHON, Language.GO, ...]:
        repo_path = get_repo_path(language)
        if repo_path.exists():
            project_name = f"test_repo_{language}"
            project = Project(
                project_root=str(repo_path),
                project_config=ProjectConfig(...),
                serena_config=config,
            )
            test_projects.append(RegisteredProject.from_project_instance(project))
    
    config.projects = test_projects
    return config

@contextmanager
def project_file_modification_context(serena_agent: SerenaAgent, relative_path: str) -> Iterator[None]:
    """Context manager to modify a project file and revert the changes after use."""
    project = serena_agent.get_active_project()
    file_path = os.path.join(project.project_root, relative_path)
    
    # Read the original content
    original_content = read_project_file(project, relative_path)
    
    try:
        yield
    finally:
        # Revert to the original content
        with open(file_path, "w", encoding=project.project_config.encoding) as f:
            f.write(original_content)
```

## Mocking

**Framework:** pytest fixtures + context managers (no external mocking library required)

**Patterns:**
```python
# Language server context setup (fixtures handle lifecycle)
with start_ls_context(Language.PYTHON) as ls:
    # Test language server operations
    symbols = ls.request_document_symbols("test_repo/models.py")

# Project context (fixtures handle cleanup)
with project_context(Language.PYTHON) as project:
    # Test project operations
    content = project.read_file("test_repo/models.py")

# File modification tracking and revert
with project_file_modification_context(serena_agent, "test_repo/models.py"):
    # Modify file
    serena_agent.activate_project("test_repo_python")
    # File reverted after context exits
```

**What to Mock:**
- Language server lifecycle: use `start_ls_context()` or `start_default_ls_context()`
- Project state: use `project_context()` or `project_with_ls_context()`
- File modifications: use `project_file_modification_context()` to ensure cleanup

**What NOT to Mock:**
- Actual language server operations (test against real test repositories)
- File system operations (use real `test/resources/repos/` test repositories)
- Symbol parsing (test with actual LSP servers)
- Tool execution (integration tests use real tools)

## Test Data & Fixtures

**Test Repositories:**
Located in `test/resources/repos/<language>/test_repo/`:
- `test/resources/repos/python/test_repo/` - Contains models.py, services.py, etc.
- `test/resources/repos/typescript/test_repo/` - Contains index.ts with DemoClass, etc.
- `test/resources/repos/go/test_repo/` - Go package structure

**Fixture utilities (from `test/solidlsp/conftest.py`):**
```python
PYTHON_BACKEND_LANGUAGES = [Language.PYTHON, Language.PYTHON_TY]

def has_malformed_name(
    symbol: UnifiedSymbolInformation,
    whitespace_allowed: bool = False,
    period_allowed: bool = False,
    colon_allowed: bool = False,
    brace_allowed: bool = False,
    parenthesis_allowed: bool = False,
    comma_allowed: bool = False,
) -> bool:
    """Check if symbol name contains forbidden characters."""
    forbidden_chars: list[str] = []
    if not whitespace_allowed:
        forbidden_chars.append(" ")
    # ... more forbidden chars
    return any(separator in symbol["name"] for separator in forbidden_chars)

def request_all_symbols(language_server: SolidLanguageServer) -> list[UnifiedSymbolInformation]:
    """Recursively get all symbols from language server."""
    result: list[UnifiedSymbolInformation] = []
    
    def visit(symbol: UnifiedSymbolInformation) -> None:
        result.append(symbol)
        for child in symbol.get("children", []):
            visit(child)
    
    symbols = language_server.request_full_symbol_tree()
    for symbol in symbols:
        visit(symbol)
    
    return result

def format_symbol_for_assert(symbol: UnifiedSymbolInformation) -> str:
    """Format symbol for test assertion error messages."""
    relative_path = symbol.get("location", {}).get("relativePath", "<unknown>")
    try:
        kind = SymbolKind(symbol["kind"]).name
    except ValueError:
        kind = str(symbol["kind"])
    return f"{symbol['name']} [{kind}] ({relative_path})"
```

## Coverage

**Requirements:** Not enforced

**View Coverage:**
```bash
# Coverage tracking through pytest, but no explicit pytest-cov configuration
# Tests are extensive with markers for language-specific and feature-specific testing
```

## Test Types

**Unit Tests:**
- Scope: Individual tool methods, utility functions, simple operations
- Approach: Test single function with real data but isolated from side effects
- Example: `test_retrieve_content_around_line()` - tests line retrieval edge cases

**Integration Tests:**
- Scope: Language server operations, project-wide symbol navigation
- Approach: Real language servers start via fixtures, test real file parsing
- Example: `test_request_references_user_class()` - uses real LSP to find references
- Language-specific: Parametrized across language variants (Python, Python-TY, etc.)

**Symbolic Editing Tests:**
- Framework: pytest with syrupy snapshots
- Location: `test/serena/test_symbol_editing.py`
- Marker: `@pytest.mark.snapshot`
- Pattern: Modify code symbols, assert output matches snapshot
- Example: Renaming class, editing method body, safe deletion

## Common Patterns

**Async Testing:**
Not heavily used; language servers handle async internally via LSP protocol.

**Error Testing:**
```python
# Symbol not found scenario
user_symbol = next((s for s in symbols[0] if s.get("name") == "User"), None)
if not user_symbol or "selectionRange" not in user_symbol:
    raise AssertionError("User symbol or its selectionRange not found")

# Edge cases
first_line_with_context_around = project.retrieve_content_around_line(file_path, 0, 2, 1)
assert len(first_line_with_context_around.lines) <= 4  # Should have at most 4 lines
for line in first_line_with_context_around.lines:
    if line.line_number == 0:
        assert line.match_type == LineType.MATCH
    elif line.line_number < 0:
        assert line.match_type == LineType.BEFORE_MATCH
    else:
        assert line.match_type == LineType.AFTER_MATCH
```

**Skip Conditions:**
```python
# Language-specific skips
Language.CLOJURE: [
    pytest.mark.clojure,
    pytest.mark.skipif(not is_clojure_cli_available(), reason="clojure CLI is not installed"),
]

# Environment skips
Language.KOTLIN: [
    pytest.mark.kotlin,
    pytest.mark.skipif(is_ci, reason="Kotlin LSP JVM crashes on restart in CI"),
]

# Tool availability skips
Language.LEAN4: [
    pytest.mark.lean4,
    pytest.mark.skipif(_sh.which("lean") is None, reason="Lean is not installed"),
]
```

## Snapshot Testing

**Framework:** syrupy 4.9.1

**File Location:** `test/serena/__snapshots__/test_symbol_editing.ambr`

**Pattern:**
```python
from syrupy import SnapshotAssertion

def test_example_edit(snapshot: SnapshotAssertion, serena_agent) -> None:
    # Perform operations
    result = serena_agent.some_operation()
    
    # Assert against snapshot
    assert result == snapshot
```

**Usage:** Symbolic editing operations (rename, edit body, delete) generate complex multi-file changes stored in snapshots for regression detection.

## Language-Server Tests Organization

**Test Repository Aliases (from `conftest.py`):**
```python
_LANGUAGE_REPO_ALIASES: dict[Language, Language] = {
    Language.CPP_CCLS: Language.CPP,
    Language.PHP_PHPACTOR: Language.PHP,
    Language.PYTHON_JEDI: Language.PYTHON,
    Language.RUBY_SOLARGRAPH: Language.RUBY,
    Language.PYTHON_TY: Language.PYTHON,
}
```

Multiple language server implementations use same test repository.

**Test Markers per Language:**
```python
_LANGUAGE_PYTEST_MARKERS: dict[Language, list[MarkDecorator | Mark]] = {
    Language.CLOJURE: [pytest.mark.clojure, pytest.mark.skipif(...)],
    Language.PYTHON: [pytest.mark.python],
    Language.RUST: [pytest.mark.rust],
    Language.TYPESCRIPT: [pytest.mark.typescript],
    # ... etc for all 40+ languages
}
```

**Multi-language testing:**
```python
@pytest.mark.parametrize("language_server", [Language.PYTHON, Language.TYPESCRIPT], indirect=True)
def test_multiple_languages(language_server: SolidLanguageServer) -> None:
    # This test will run once for each language
    pass
```

## CI/Environment Handling

**CI Detection (from `conftest.py`):**
```python
is_ci = os.getenv("CI") == "true" or os.getenv("GITHUB_ACTIONS") == "true"
is_windows = platform.system() == "Windows"
```

**Language Availability (from `conftest.py`):**
```python
def _determine_disabled_languages() -> list[Language]:
    """Determine which language tests should be disabled (based on the environment)"""
    result: list[Language] = []
    
    # Disable Java tests if not available
    java_tests_enabled = True
    if not java_tests_enabled:
        result.append(Language.JAVA)
    
    # Disable CPP_CCLS tests if ccls is not available
    ccls_tests_enabled = _sh.which("ccls") is not None
    if not ccls_tests_enabled:
        result.append(Language.CPP_CCLS)
    
    return result

_disabled_languages = _determine_disabled_languages()

def language_tests_enabled(language: Language) -> bool:
    """Check if tests for the given language are enabled in the current environment."""
    return language not in _disabled_languages
```

---

*Testing analysis: 2026-04-07*
