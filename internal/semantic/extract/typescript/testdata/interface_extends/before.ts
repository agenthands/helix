interface Reader {
    read(): number;
}
interface Closer extends Reader {
    close(): void;
}
