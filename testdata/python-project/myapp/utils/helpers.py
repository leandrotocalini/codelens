import os
from datetime import datetime

class Logger:
    def __init__(self, name):
        self.name = name

    def log(self, message):
        print(f"[{self.name}] {message}")

def format_timestamp(dt):
    return dt.isoformat()

def _private_helper():
    pass
