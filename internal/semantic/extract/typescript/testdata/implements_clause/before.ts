interface Reader {
    read(): number;
}
class FileReader implements Reader {
    read(): number { return 0; }
}
