from ctypes import cdll, c_char_p, c_int
import os

class GoServer:
    def __init__(self):
        try:
            self.lib = cdll.LoadLibrary("./libgoserver.so")
            # No need to set argtypes for uintptr explicitly as c_char_p works as a pointer
            self.lib.RegisterRoute.argtypes = [c_char_p, c_char_p, c_char_p, c_char_p]
            self.lib.RegisterMiddleware.argtypes = [c_char_p, c_int]
            self.lib.RegisterDependency.argtypes = [c_char_p, c_char_p]
        except OSError as e:
            raise RuntimeError(f"Failed to load libgoserver.so: {e}")

    def route(self, path, method="GET", description=""):
        def decorator(func):
            self.lib.RegisterRoute(
                path.encode('utf-8'),
                method.encode('utf-8'),
                func().encode('utf-8'),
                description.encode('utf-8')
            )
            return func
        return decorator

    def middleware(self, name, enabled=True):
        self.lib.RegisterMiddleware(name.encode('utf-8'), c_int(1 if enabled else 0))

    def dependency(self, name, value):
        self.lib.RegisterDependency(name.encode('utf-8'), value.encode('utf-8'))

    def start(self):
        self.lib.StartServer()