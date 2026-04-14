const { Greeter } = require("./greeter");

function helper() {
    return "hello";
}

class DemoClass {
    constructor(value) {
        this.value = value;
    }

    getValue() {
        return this.value;
    }
}

function unusedFunc() {}

function usingHelper() {
    return helper();
}

const g = new Greeter();
console.log(g.greet("World"));
console.log(helper());
