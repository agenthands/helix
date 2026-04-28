import json
import subprocess
import time

def call_tools_sequential(calls):
    proc = subprocess.Popen(
        ["/Users/Janis_Vizulis/go/bin/serena", "--mode=stdio"],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True
    )
    
    # MCP initialize
    init_msg = {
        "jsonrpc": "2.0",
        "id": 0,
        "method": "initialize",
        "params": {"protocolVersion": "2024-11-05", "capabilities": {}, "clientInfo": {"name": "test", "version": "1.0"}}
    }
    proc.stdin.write(json.dumps(init_msg) + "\n")
    
    results = []
    for i, (method, params) in enumerate(calls):
        msg = {"jsonrpc": "2.0", "id": i+1, "method": method, "params": params}
        proc.stdin.write(json.dumps(msg) + "\n")
    
    proc.stdin.flush()
    
    # Read responses
    try:
        # Expect len(calls) + 1 lines
        for _ in range(len(calls) + 1):
            line = proc.stdout.readline()
            if line:
                results.append(json.loads(line))
    except Exception as e:
        results.append({"error": str(e)})
        
    proc.terminate()
    return results

calls = [
    ("tools/call", {"name": "activate_project", "arguments": {"repo_path": "/Users/Janis_Vizulis/go/src/github.com/postfix/serena"}}),
    ("tools/call", {"name": "search_symbols", "arguments": {"query": "DefaultSocketPath"}})
]

results = call_tools_sequential(calls)
for res in results:
    print(json.dumps(res, indent=2))
