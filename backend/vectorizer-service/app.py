from typing import List

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field
from sentence_transformers import SentenceTransformer


class EmbedRequest(BaseModel):
    texts: List[str] = Field(default_factory=list)
    model: str | None = None


class EmbedResponse(BaseModel):
    vectors: List[List[float]]


app = FastAPI(title="cgoforum-sentence-transformer-service")
_model_cache: dict[str, SentenceTransformer] = {}


def get_model(name: str) -> SentenceTransformer:
    if name not in _model_cache:
        _model_cache[name] = SentenceTransformer(name)
    return _model_cache[name]


@app.get("/health")
def health() -> dict:
    return {"status": "ok"}


@app.post("/embed", response_model=EmbedResponse)
def embed(req: EmbedRequest) -> EmbedResponse:
    if not req.texts:
        raise HTTPException(status_code=400, detail="texts is required")

    model_name = req.model or "moka-ai/m3e-base"
    model = get_model(model_name)
    vectors = model.encode(req.texts, normalize_embeddings=True).tolist()
    return EmbedResponse(vectors=vectors)
