APP_NAME = "PersonalDashboard"
REPO = "KrustyLeBot/PersonalApp"

try:
    # Written by the CI before packaging; absent when running from source.
    from ._build import BUILD
except ImportError:
    BUILD = 0

VERSION = f"0.1.{BUILD}"
