function outer(): () => number {
    return () => 42;
}
