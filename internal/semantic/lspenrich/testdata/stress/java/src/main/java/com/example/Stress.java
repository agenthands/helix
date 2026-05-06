package com.example;

/**
 * Stress is the caller-side class for the Phase 61 P04 ENRICH-05 stress
 * fixture (Java). Stress.run() calls Lib.value() so callHierarchy on
 * Stress.run yields one CALLS edge to Lib.value.
 */
public class Stress {
    private final Lib lib;

    public Stress(Lib lib) {
        this.lib = lib;
    }

    public String run(String name) {
        return name + ":" + lib.value();
    }

    public String runWithDefault() {
        return run("default");
    }
}
