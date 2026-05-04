function mapList<T, U>(xs: T[], f: (t: T) => U): U[] {
    return xs.map(f);
}
