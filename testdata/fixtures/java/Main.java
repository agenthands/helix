public class Main {
    public static String helper() {
        return "hello";
    }

    public static void unusedMethod() {}

    public static String usingHelper() {
        return helper();
    }

    public static void main(String[] args) {
        Greeter g = new SimpleGreeter();
        System.out.println(g.greet("World"));
        System.out.println(helper());
    }
}
