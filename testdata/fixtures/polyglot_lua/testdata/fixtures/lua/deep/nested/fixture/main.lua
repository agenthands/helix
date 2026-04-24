-- main.lua — deeply nested Lua testdata fixture.
local calculator = require('src.calculator')
local utils = require('src.utils')

local function run()
    local r = calculator.add(1, 2)
    r = calculator.subtract(r, 1)
    r = calculator.multiply(r, 3)
    utils.log('result: ' .. tostring(r))
    local parts = utils.split('a,b,c', ',')
    for _, p in ipairs(parts) do
        utils.log(utils.trim(p))
    end
    local logger = utils.Logger.new()
    logger:log('done')
end

run()
