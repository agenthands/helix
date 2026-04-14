const std = @import("std");
const greeter = @import("greeter.zig");

pub fn helper() []const u8 {
    return "hello";
}

const DemoStruct = struct {
    value: i32,

    pub fn getValue(self: DemoStruct) i32 {
        return self.value;
    }
};

fn unusedFunc() void {}

pub fn usingHelper() []const u8 {
    return helper();
}

pub fn main() !void {
    const stdout = std.io.getStdOut().writer();
    try stdout.print("{s}\n", .{greeter.greet("World")});
    try stdout.print("{s}\n", .{helper()});
}
