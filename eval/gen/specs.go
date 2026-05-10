package main

// builtinSpecs returns the 20 generated corpus tasks.
//
// Distribution (matches PLAN <interfaces> table):
//
//	Go (7):  rename × 3, delete × 2, public_api × 2
//	TS (6):  rename × 2, delete × 2, public_api × 2
//	Py (7):  rename × 2, delete × 2, public_api × 3
//
// Existing 10 hand-authored tasks (go-rename-public-001 etc.) are NOT in
// this table and are NOT touched by the generator.
func builtinSpecs() []TaskSpec {
	return []TaskSpec{
		// ── Go × rename (3) ──────────────────────────────────────────────
		{
			ID: "go-rename-002", Lang: Go, Family: FamRename,
			OldSymbol: "computeChecksum", NewSymbol: "calculateChecksum",
			Caller:      "ProcessPayload",
			Doc:         "computes a checksum over the payload bytes (private helper).",
			Instruction: "Rename the private function `computeChecksum` to `calculateChecksum` throughout the package. Update all callers. The package must still vet clean.",
			SeedDecl: `func computeChecksum(payload []byte) int {
	sum := 0
	for _, b := range payload {
		sum += int(b)
	}
	return sum
}`,
			SeedCaller: `func ProcessPayload(payload []byte) string {
	c := computeChecksum(payload)
	return fmt.Sprintf("checksum=%d", c)
}`,
			MainArgs: `[]byte("hello")`,
		},
		{
			ID: "go-rename-003", Lang: Go, Family: FamRename,
			OldSymbol: "Dispatch", NewSymbol: "Send",
			Caller:      "Broadcast",
			Doc:         "dispatches a message to a single channel (method on Router).",
			Instruction: "Rename the method `Dispatch` on type `Router` to `Send`. Update all call sites. The package must still vet clean.",
			SeedDecl: `type Router struct{ name string }

func (r *Router) Dispatch(msg string) string {
	return r.name + ": " + msg
}`,
			SeedCaller: `func Broadcast(r *Router, msgs []string) []string {
	out := make([]string, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, r.Dispatch(m))
	}
	return out
}`,
			MainArgs: `&Router{name: "main"}, []string{"hi"}`,
		},
		{
			ID: "go-rename-004", Lang: Go, Family: FamRename,
			OldSymbol: "MaxRetries", NewSymbol: "RetryLimit",
			Caller:      "ShouldRetry",
			Doc:         "is the maximum retry count (struct field on Config).",
			Instruction: "Rename the struct field `MaxRetries` on type `Config` to `RetryLimit`. Update all readers. The package must still vet clean.",
			SeedDecl: `type Config struct {
	MaxRetries int
}`,
			SeedCaller: `func ShouldRetry(c Config, attempt int) bool {
	return attempt < c.MaxRetries
}`,
			MainArgs: `Config{MaxRetries: 3}, 1`,
		},

		// ── Go × delete (2) ──────────────────────────────────────────────
		{
			ID: "go-delete-002", Lang: Go, Family: FamDelete,
			OldSymbol: "unusedHelper", Caller: "RealWork",
			Doc:         "is an unused legacy helper (deprecated).",
			Instruction: "Remove the unused private function `unusedHelper`. The function has no callers — delete it cleanly. The package must still vet clean.",
			SeedDecl: `func unusedHelper(s string) string {
	return "[" + s + "]"
}`,
			SeedCaller: `func RealWork(s string) string {
	return s + "!"
}`,
			MainArgs: `"hi"`,
			PostEditFull: `package main

import (
	"fmt"
)

// RealWork is the active helper.
func RealWork(s string) string {
	return s + "!"
}

func main() {
	result := RealWork("hi")
	fmt.Println(result)
}
`,
		},
		{
			ID: "go-delete-003", Lang: Go, Family: FamDelete,
			OldSymbol: "DeprecatedFormat", Caller: "FormatItem",
			Doc:         "is a deprecated format method on type Item.",
			Instruction: "Remove the deprecated method `DeprecatedFormat` on type `Item`. Update its single caller `FormatItem` to use `NewFormat` instead. The package must still vet clean.",
			SeedDecl: `type Item struct{ name string }

func (i Item) DeprecatedFormat() string {
	return "old:" + i.name
}

func (i Item) NewFormat() string {
	return "new:" + i.name
}`,
			SeedCaller: `func FormatItem(i Item) string {
	return i.DeprecatedFormat()
}`,
			MainArgs: `Item{name: "x"}`,
			PostEditFull: `package main

import (
	"fmt"
)

type Item struct{ name string }

func (i Item) NewFormat() string {
	return "new:" + i.name
}

// FormatItem calls NewFormat.
func FormatItem(i Item) string {
	return i.NewFormat()
}

func main() {
	result := FormatItem(Item{name: "x"})
	fmt.Println(result)
}
`,
		},

		// ── Go × public_api (2) ──────────────────────────────────────────
		{
			ID: "go-public-api-002", Lang: Go, Family: FamPublicAPI,
			OldSymbol: "Encode", Caller: "RunEncoder",
			Doc:               "encodes the input. Will be widened to support a context.Context first parameter.",
			Instruction:       "Change the signature of the public method `Encode` on type `Encoder` to accept a `context.Context` as its first parameter. Analyze blast radius before editing. Update all callers (and add the `context` import). The package must still vet clean.",
			GoPostEditImports: []string{"context"},
			SeedDecl: `type Encoder struct{}

func (e *Encoder) Encode(input string) string {
	return "encoded:" + input
}`,
			SeedCaller: `func RunEncoder(e *Encoder, input string) string {
	return e.Encode(input)
}`,
			MainArgs: `&Encoder{}, "hi"`,
			Sentinel: "context.Context",
			PostEditDecl: `type Encoder struct{}

func (e *Encoder) Encode(ctx context.Context, input string) string {
	_ = ctx
	return "encoded:" + input
}`,
			PostEditCaller: `func RunEncoder(e *Encoder, input string) string {
	return e.Encode(context.Background(), input)
}`,
		},
		{
			ID: "go-public-api-003", Lang: Go, Family: FamPublicAPI,
			OldSymbol: "Validate", Caller: "RunValidate",
			Doc:         "validates input and returns an error. Will be widened to return a typed *ValidationError instead of error.",
			Instruction: "Change the public function `Validate` so it returns a typed `*ValidationError` (declared in the package) instead of a generic `error`. Analyze blast radius before editing. Update all callers. The package must still vet clean.",
			SeedDecl: `func Validate(input string) error {
	if input == "" {
		return fmt.Errorf("empty input")
	}
	return nil
}`,
			SeedCaller: `func RunValidate(input string) string {
	if err := Validate(input); err != nil {
		return "invalid: " + err.Error()
	}
	return "ok"
}`,
			MainArgs: `"hi"`,
			Sentinel: "ValidationError",
			PostEditDecl: `type ValidationError struct{ Reason string }

func (e *ValidationError) Error() string { return e.Reason }

func Validate(input string) *ValidationError {
	if input == "" {
		return &ValidationError{Reason: "empty input"}
	}
	return nil
}`,
		},

		// ── TS × rename (2) ──────────────────────────────────────────────
		{
			ID: "ts-rename-002", Lang: TS, Family: FamRename,
			OldSymbol: "encodeMessage", NewSymbol: "serializeMessage",
			Caller:      "sendBatch",
			Doc:         "is a method of the Channel class that encodes a message string.",
			Instruction: "Rename the method `encodeMessage` on class `Channel` to `serializeMessage`. Update all call sites. No occurrences of `encodeMessage` may remain.",
			SeedDecl: `export class Channel {
  prefix: string;
  constructor(prefix: string) { this.prefix = prefix; }
  encodeMessage(msg: string): string {
    return this.prefix + ":" + msg;
  }
}`,
			SeedCaller: `export function sendBatch(c: Channel, msgs: string[]): string[] {
  return msgs.map((m) => c.encodeMessage(m));
}`,
		},
		{
			ID: "ts-rename-003", Lang: TS, Family: FamRename,
			OldSymbol: "DEFAULT_TIMEOUT", NewSymbol: "DEFAULT_TIMEOUT_MS",
			Caller:      "withTimeout",
			Doc:         "is the exported default timeout constant.",
			Instruction: "Rename the exported constant `DEFAULT_TIMEOUT` to `DEFAULT_TIMEOUT_MS`. Update all readers. No occurrences of the old name may remain.",
			SeedDecl:    `export const DEFAULT_TIMEOUT = 5000;`,
			SeedCaller: `export function withTimeout(custom?: number): number {
  return custom ?? DEFAULT_TIMEOUT;
}`,
		},

		// ── TS × delete (2) ──────────────────────────────────────────────
		{
			ID: "ts-delete-002", Lang: TS, Family: FamDelete,
			OldSymbol: "unusedExport", Caller: "activeExport",
			Doc:         "is an unused legacy export.",
			Instruction: "Remove the unused exported function `unusedExport` from the module. No occurrences may remain.",
			SeedDecl: `export function unusedExport(x: number): number {
  return x * 2;
}`,
			SeedCaller: `export function activeExport(x: number): number {
  return x + 1;
}`,
			PostEditFull: `// Seed module — legacy export removed.
export function activeExport(x: number): number {
  return x + 1;
}
`,
		},
		{
			ID: "ts-delete-003", Lang: TS, Family: FamDelete,
			OldSymbol: "deadBranchHandler", Caller: "dispatch",
			Doc:         "is a dead branch handler that is never reached.",
			Instruction: "Remove the dead function `deadBranchHandler` and the conditional branch in `dispatch` that calls it. The remaining dispatch logic must still type-check.",
			SeedDecl: `export function deadBranchHandler(x: number): string {
  return "dead:" + x;
}`,
			SeedCaller: `export function dispatch(x: number, useDead: boolean): string {
  if (useDead) {
    return deadBranchHandler(x);
  }
  return "live:" + x;
}`,
			PostEditFull: `// Seed module — dead branch removed; dispatch simplified.
export function dispatch(x: number, _useDead: boolean): string {
  return "live:" + x;
}
`,
		},

		// ── TS × public_api (2) ──────────────────────────────────────────
		{
			ID: "ts-public-api-002", Lang: TS, Family: FamPublicAPI,
			OldSymbol: "loadConfig", Caller: "bootstrap",
			Doc:         "loads a config and returns a string. Will be widened to return a Config object.",
			Instruction: "Widen the return type of the exported function `loadConfig` from `string` to `{ name: string; version: number }`. Analyze blast radius first. Update all callers.",
			SeedDecl: `export function loadConfig(name: string): string {
  return name;
}`,
			SeedCaller: `export function bootstrap(name: string): string {
  const cfg = loadConfig(name);
  return "loaded:" + cfg;
}`,
			Sentinel: "version",
			PostEditDecl: `export function loadConfig(name: string): { name: string; version: number } {
  return { name, version: 1 };
}`,
		},
		{
			ID: "ts-public-api-003", Lang: TS, Family: FamPublicAPI,
			OldSymbol: "computeTotal", Caller: "summarize",
			Doc:         "computes a total. Will accept a new options parameter.",
			Instruction: "Change the signature of the exported function `computeTotal` to accept a second parameter `opts: { tax: number }`. Analyze blast radius first. Update all callers.",
			SeedDecl: `export function computeTotal(items: number[]): number {
  return items.reduce((a, b) => a + b, 0);
}`,
			SeedCaller: `export function summarize(items: number[]): string {
  const total = computeTotal(items);
  return "total=" + total;
}`,
			Sentinel: "opts",
			PostEditDecl: `export function computeTotal(items: number[], opts: { tax: number }): number {
  const sub = items.reduce((a, b) => a + b, 0);
  return sub + opts.tax;
}`,
		},

		// ── Py × rename (2) ──────────────────────────────────────────────
		{
			ID: "py-rename-002", Lang: Python, Family: FamRename,
			OldSymbol: "DataLoader", NewSymbol: "DataReader",
			Caller:      "load_all",
			Doc:         "Rename DataLoader class to DataReader.",
			Instruction: "Rename the class `DataLoader` to `DataReader` throughout the module. Update all references. No occurrences of `DataLoader` may remain.",
			SeedDecl: `class DataLoader:
    def __init__(self, source: str) -> None:
        self.source = source

    def read(self) -> str:
        return f"data from {self.source}"`,
			SeedCaller: `def load_all(sources: list[str]) -> list[str]:
    return [DataLoader(s).read() for s in sources]`,
			MainArgs: `["a", "b"]`,
		},
		{
			ID: "py-rename-003", Lang: Python, Family: FamRename,
			OldSymbol: "MAX_BUFFER", NewSymbol: "MAX_BUFFER_BYTES",
			Caller:      "read_chunk",
			Doc:         "Rename the module-level constant MAX_BUFFER to MAX_BUFFER_BYTES.",
			Instruction: "Rename the module-level constant `MAX_BUFFER` to `MAX_BUFFER_BYTES`. Update all readers.",
			SeedDecl:    `MAX_BUFFER = 4096`,
			SeedCaller: `def read_chunk(stream) -> bytes:
    return stream.read(MAX_BUFFER)`,
			MainArgs: `_StubStream()`,
		},

		// ── Py × delete (2) ──────────────────────────────────────────────
		{
			ID: "py-delete-001", Lang: Python, Family: FamDelete,
			OldSymbol: "_legacy_helper", Caller: "current_helper",
			Doc:         "Delete the unused legacy helper.",
			Instruction: "Remove the unused private helper `_legacy_helper` from the module. No occurrences may remain.",
			SeedDecl: `def _legacy_helper(s: str) -> str:
    return f"[{s}]"`,
			SeedCaller: `def current_helper(s: str) -> str:
    return s + "!"`,
			MainArgs: `"hi"`,
			PostEditFull: `"""Seed module for py-delete-001 — legacy helper removed."""


def current_helper(s: str) -> str:
    return s + "!"


if __name__ == "__main__":
    print(current_helper("hi"))
`,
		},
		{
			ID: "py-delete-002", Lang: Python, Family: FamDelete,
			OldSymbol: "OldName", Caller: "use_alias",
			Doc:         "Delete the legacy alias OldName.",
			Instruction: "Remove the legacy alias `OldName = NewName`. Update `use_alias` to reference `NewName` directly. No occurrences of `OldName` may remain.",
			SeedDecl: `class NewName:
    def value(self) -> str:
        return "value"


OldName = NewName`,
			SeedCaller: `def use_alias() -> str:
    return OldName().value()`,
			MainArgs: ``,
			PostEditFull: `"""Seed module for py-delete-002 — legacy alias removed."""


class NewName:
    def value(self) -> str:
        return "value"


def use_alias() -> str:
    return NewName().value()


if __name__ == "__main__":
    print(use_alias())
`,
		},

		// ── Py × public_api (3) ──────────────────────────────────────────
		{
			ID: "py-public-api-002", Lang: Python, Family: FamPublicAPI,
			OldSymbol: "fetch_user", Caller: "summarize_user",
			Doc:         "Add a required user_id keyword parameter.",
			Instruction: "Change the signature of `fetch_user` to accept a new keyword parameter `user_id: str`. Analyze blast radius first. Update all callers.",
			SeedDecl: `def fetch_user(name: str) -> dict[str, str]:
    return {"name": name}`,
			SeedCaller: `def summarize_user(name: str) -> str:
    user = fetch_user(name)
    return f"user={user['name']}"`,
			MainArgs: `"alice"`,
			Sentinel: "user_id",
			PostEditDecl: `def fetch_user(name: str, user_id: str = "") -> dict[str, str]:
    return {"name": name, "user_id": user_id}`,
		},
		{
			ID: "py-public-api-003", Lang: Python, Family: FamPublicAPI,
			OldSymbol: "compute_score", Caller: "report_score",
			Doc:         "Change the default value of bonus from 0 to 10.",
			Instruction: "Change the default value of the `bonus` parameter on `compute_score` from `0` to `10`. Analyze blast radius first; the new default must appear as `bonus: int = 10`.",
			SeedDecl: `def compute_score(base: int, bonus: int = 0) -> int:
    return base + bonus`,
			SeedCaller: `def report_score(base: int) -> str:
    return f"score={compute_score(base)}"`,
			MainArgs: `5`,
			Sentinel: "bonus: int = 10",
			PostEditDecl: `def compute_score(base: int, bonus: int = 10) -> int:
    return base + bonus`,
		},
		{
			ID: "py-public-api-004", Lang: Python, Family: FamPublicAPI,
			OldSymbol: "parse_input", Caller: "load_input",
			Doc:         "Change the raised exception type from ValueError to InputError.",
			Instruction: "Change `parse_input` to raise a new exception type `InputError` (declared in the module) instead of `ValueError`. Analyze blast radius first. Update all callers.",
			SeedDecl: `def parse_input(s: str) -> int:
    if not s:
        raise ValueError("empty input")
    return int(s)`,
			SeedCaller: `def load_input(s: str) -> int:
    try:
        return parse_input(s)
    except ValueError:
        return -1`,
			MainArgs: `"42"`,
			Sentinel: "InputError",
			PostEditDecl: `class InputError(Exception):
    pass


def parse_input(s: str) -> int:
    if not s:
        raise InputError("empty input")
    return int(s)`,
		},
	}
}
