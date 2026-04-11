export class Config {
    constructor(public name: string) {}
}

export function Handler(c: Config): string {
    return c.name;
}

export function Parse(raw: string): Config {
    return new Config(raw);
}
