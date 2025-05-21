import requests
import json
import time

url = 'https://core.inferoute.com/v1/chat/completions'
headers = {
    'Content-Type': 'application/json',
    'Authorization': 'Bearer sk-5d63a42ace77f48e8c39713f0c915d87',
    'Accept': 'text/event-stream'  # Explicitly accept event stream
}
data = {
    #"model": "deepseek-ai/DeepSeek-R1-Distill-Qwen-1.5B",
    "model": "gguf/deepseek-r1:8b",
    "messages": [
      {
        "role": "system",
        "content": "You are a helpful assistant."
      },
      {
        "role": "user",
        "content": "What is the capital of France?"
      }
    ],
    "max_tokens": 1000,
    "temperature": 0.7,
    "sort": "throughput",
    "stream": True  # Ensure stream is True in the payload
}

try:
    with requests.post(url, headers=headers, json=data, stream=True) as response:
        response.raise_for_status()  # Raise an exception for bad status codes
        print(f"Connected. Streaming response (Status: {response.status_code}):\n")
        for line in response.iter_lines():
            if line:
                decoded_line = line.decode('utf-8')
                print(decoded_line)
                time.sleep(1)
except requests.exceptions.RequestException as e:
    print(f"Request failed: {e}")
except Exception as e:
    print(f"An unexpected error occurred: {e}") 