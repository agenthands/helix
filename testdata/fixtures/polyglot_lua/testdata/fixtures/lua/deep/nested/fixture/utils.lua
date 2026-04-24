-- utils.lua — Lua fixture with bare-name defs (log, trim, split, new,
-- Logger) designed to collide with Go bare refs on the unfixed codebase.
local utils = {}

function utils.log(msg)
    io.write(tostring(msg) .. '\n')
end

function utils.trim(s)
    return (s:gsub('^%s*(.-)%s*$', '%1'))
end

function utils.split(s, sep)
    local out = {}
    for part in string.gmatch(s, '([^' .. sep .. ']+)') do
        out[#out + 1] = part
    end
    return out
end

utils.Logger = {}

function utils.Logger.new()
    local self = setmetatable({}, { __index = utils.Logger })
    return self
end

function utils.Logger:log(msg)
    utils.log(msg)
end

return utils
