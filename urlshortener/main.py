import argparse
import sys
from threading import Lock
from typing import Dict

from fastapi import FastAPI, HTTPException
from fastapi.responses import RedirectResponse
from pydantic import BaseModel

# Глобальные переменные для хранения данных
key_to_url: Dict[str, str] = {}  # key -> URL
url_to_key: Dict[str, str] = {}  # URL -> key (для проверки дубликатов)
counter: int = 0
mutex = Lock()

BASE62_CHARS = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

app = FastAPI()


class ShortenRequest(BaseModel):
    """Модель запроса для /shorten."""
    url: str


class ShortenResponse(BaseModel):
    """Модель ответа для /shorten."""
    url: str
    key: str


def int_to_base62(n: int) -> str:
    """Конвертирует число в base62 строку."""
    if n == 0:
        return "0"
    result = []
    while n > 0:
        result.insert(0, BASE62_CHARS[n % 62])
        n //= 62
    return "".join(result)


@app.get("/pong")
def pong_handler():
    """Обработчик для /pong эндпоинта."""
    return {"message": "pong"}


@app.post("/shorten", response_model=ShortenResponse)
def shorten_handler(request: ShortenRequest):
    """Обработчик для /shorten эндпоинта."""
    url = request.url
    print(f"Parsed URL: {url}")
    
    with mutex:
        # Проверяем, есть ли уже такой URL
        if url in url_to_key:
            key = url_to_key[url]
        else:
            # Генерируем новый ключ
            global counter
            counter += 1
            key = int_to_base62(counter)
            url_to_key[url] = key
            key_to_url[key] = url
    
    return {"url": url, "key": key}


@app.get("/go/{key}")
def go_handler(key: str):
    """Обработчик для /go/{key} эндпоинта."""
    with mutex:
        if key not in key_to_url:
            raise HTTPException(status_code=404, detail="key not found")
        url = key_to_url[key]
    
    return RedirectResponse(url=url, status_code=302)


def get_port() -> int:
    """Парсит аргументы командной строки и возвращает порт."""
    parser = argparse.ArgumentParser()
    parser.add_argument("-port", type=int, required=True, help="port number")
    args = parser.parse_args()
    
    if args.port < 1 or args.port > 65535:
        print(f"Error: invalid port number: {args.port}", file=sys.stderr)
        sys.exit(1)
    
    print(f"port: {args.port}")
    return args.port


if __name__ == "__main__":
    import uvicorn
    
    port = get_port()
    uvicorn.run(app, host="0.0.0.0", port=port)

