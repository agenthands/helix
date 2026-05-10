The public function `process_batch` needs a new optional parameter `timeout_sec`
with a default of 30. Analyze the blast radius before making the change, then
update the function signature. All existing callers should continue to work
without modification.
