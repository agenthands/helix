<?php
require_once 'greeter.php';

function helper(): string {
    return "hello";
}

class DemoClass {
    private int $value;

    public function __construct(int $value) {
        $this->value = $value;
    }

    public function getValue(): int {
        return $this->value;
    }
}

function unusedFunc(): void {}

function usingHelper(): string {
    return helper();
}

$g = new Greeter();
echo $g->greet("World") . "\n";
echo helper() . "\n";
