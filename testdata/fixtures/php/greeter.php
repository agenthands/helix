<?php

interface Greetable {
    public function greet(string $name): string;
}

class Greeter implements Greetable {
    public function greet(string $name): string {
        return "Hello, {$name}!";
    }
}
