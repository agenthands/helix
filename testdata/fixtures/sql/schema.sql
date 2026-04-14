-- Cross-file reference: greeter function
CREATE FUNCTION greet(person_name TEXT) RETURNS TEXT AS $$
BEGIN
    RETURN 'Hello, ' || person_name || '!';
END;
$$ LANGUAGE plpgsql;
