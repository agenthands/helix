interface Greeter {
    String greet(String name);
}

class SimpleGreeter implements Greeter {
    @Override
    public String greet(String name) {
        return "Hello, " + name + "!";
    }
}
