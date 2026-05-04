def logged(fn):
    return fn

@logged
def hello():
    return "hi"
