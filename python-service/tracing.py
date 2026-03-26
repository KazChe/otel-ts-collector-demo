import os

from opentelemetry import trace
from opentelemetry.sdk import trace as trace_sdk
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.instrumentation.httpx import HTTPXClientInstrumentor
from openinference.instrumentation.langchain import LangChainInstrumentor


def init_tracing(app):
    endpoint = os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")

    resource = Resource.create(
        {
            "service.name": os.getenv(
                "OTEL_SERVICE_NAME", "python-langgraph-service"
            ),
            "service.version": "1.0.0",
        }
    )

    provider = trace_sdk.TracerProvider(resource=resource)

    # Export spans to OTEL Collector (which forwards to Galileo + debug)
    exporter = OTLPSpanExporter(endpoint=f"{endpoint}/v1/traces")
    provider.add_span_processor(BatchSpanProcessor(exporter))

    trace.set_tracer_provider(provider)

    # Auto-instrument frameworks
    # FastAPI: creates server spans for inbound requests
    FastAPIInstrumentor.instrument_app(app)
    # httpx: injects traceparent header into outbound HTTP calls
    HTTPXClientInstrumentor().instrument()
    # LangChain/LangGraph: adds gen_ai.* attributes to graph execution spans
    LangChainInstrumentor().instrument(tracer_provider=provider)

    print(f"[tracing] OpenTelemetry initialized, exporting to {endpoint}")
