import { Greeter } from "./greeter";

export function helper(): string {
    return "hello";
}

export class DemoClass {
    constructor(public value: number) {}

    getValue(): number {
        return this.value;
    }
}

const g = new Greeter();
console.log(g.greet("World"));
