#include "greeter.h"
#include <string>
#include <iostream>

std::string helper() {
    return "hello";
}

struct DemoStruct {
    int value;

    int get_value() const {
        return value;
    }
};

void unused_func() {}

std::string using_helper() {
    return helper();
}

int main() {
    Greeter g;
    std::cout << g.greet("World") << std::endl;
    std::cout << helper() << std::endl;
    return 0;
}
