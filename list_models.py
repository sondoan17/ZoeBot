from google import genai
import os
from dotenv import load_dotenv

load_dotenv()
api_key = os.getenv('GEMINI_API_KEY')

if not api_key:
    print("No API key found")
else:
    client = genai.Client(api_key=api_key)
    try:
        # Pager object, iterate to get models
        for m in client.models.list():
            print(f"Model: {m.name}")
    except Exception as e:
        print(f"Error: {e}")
