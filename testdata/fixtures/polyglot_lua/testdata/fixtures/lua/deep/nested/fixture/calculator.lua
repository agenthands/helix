-- calculator.lua — Lua fixture with bare-name defs intended to collide
-- with Go bare refs in the pre-fix codebase. Post-fix, F1-B/F1-A dampen
-- and qualify the collisions so Go pkg/ still wins top-rank.
local calculator = {}

function calculator.add(a, b)
    return a + b
end

function calculator.subtract(a, b)
    return a - b
end

function calculator.multiply(a, b)
    return a * b
end

function calculator.divide(a, b)
    if b == 0 then return nil end
    return a / b
end

function calculator.power(a, b)
    return a ^ b
end

return calculator
