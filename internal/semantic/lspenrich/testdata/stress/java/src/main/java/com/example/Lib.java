package com.example;

/**
 * Lib is the callee-side class for the Phase 61 P04 ENRICH-05 stress
 * fixture (Java). Lib.value() is the target of Stress.run()'s
 * callHierarchy edge.
 */
public class Lib {
    private final String tag;

    public Lib(String tag) {
        this.tag = tag;
    }

    public String value() {
        return tag;
    }
}
