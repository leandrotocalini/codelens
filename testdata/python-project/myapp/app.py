from flask import Flask
from myapp.utils import helpers

class Application:
    def __init__(self, config):
        self.config = config
        self.app = Flask(__name__)

    def run(self):
        self.app.run()

def create_app(config=None):
    return Application(config)

def _internal_setup():
    pass
