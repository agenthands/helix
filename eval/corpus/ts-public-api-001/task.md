The exported function `createSession` needs to return a `Promise<Session>`
instead of a plain `Session`. Analyze the blast radius before making the
change, then update the function signature and all call sites.
