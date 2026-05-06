package com.example;

/**
 * A is the caller in the cascade integration-test fixture. A.method() calls
 * B.method() so callHierarchy on A.method yields one CALLS edge.
 */
public class A {
    public String method() {
        B b = new B();
        return b.method();
    }
}
