class Bar:
    def hello(self) -> str:
        return "hi"


def main() -> None:
    obj = Bar()
    print(obj.hello())


if __name__ == "__main__":
    main()
