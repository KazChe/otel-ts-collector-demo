import os

from fastapi import FastAPI
from pydantic import BaseModel
import httpx
from langgraph.graph import StateGraph, START, END
from typing import TypedDict

app = FastAPI(title="Service A - Python/LangGraph")

from tracing import init_tracing

init_tracing(app)

TS_SERVICE_URL = os.getenv("TS_SERVICE_URL", "http://localhost:3000")


class AgentState(TypedDict):
    question: str
    ts_response: str
    final_answer: str


async def call_ts_service(state: AgentState) -> dict:
    """Call Service B (TypeScript) — httpx auto-injects traceparent header."""
    async with httpx.AsyncClient() as client:
        resp = await client.post(
            f"{TS_SERVICE_URL}/agent",
            json={"message": state["question"]},
            timeout=30.0,
        )
        resp.raise_for_status()
        data = resp.json()
    return {"ts_response": data.get("answer", "")}


async def format_response(state: AgentState) -> dict:
    """Wrap the response from Service B."""
    return {"final_answer": f"[LangGraph orchestrated] {state['ts_response']}"}


# Build LangGraph: START → call_ts_service → format_response → END
builder = StateGraph(AgentState)
builder.add_node("call_ts_service", call_ts_service)
builder.add_node("format_response", format_response)
builder.add_edge(START, "call_ts_service")
builder.add_edge("call_ts_service", "format_response")
builder.add_edge("format_response", END)
graph = builder.compile()


class AskRequest(BaseModel):
    question: str


@app.post("/ask")
async def ask(req: AskRequest):
    result = await graph.ainvoke(
        {"question": req.question, "ts_response": "", "final_answer": ""}
    )
    return {"answer": result["final_answer"]}


@app.get("/health")
async def health():
    return {"status": "ok"}
