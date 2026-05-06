package com.example;

/**
 * B is the callee in the cascade integration-test fixture. B.method() is
 * the target of A.method()'s callHierarchy edge.
 */
public class B {
    public String method() {
        return "from B";
    }
}
