import dotenv from "dotenv";
dotenv.config();

// initTracing MUST be called before importing Express
// so HTTP instrumentation can monkey-patch the http module.
// We use dynamic import() because ES module static imports are hoisted.
import { initTracing, shutdownTracing } from "./tracing.js";
initTracing();

const { default: express } = await import("express");
const { runConversationTurn } = await import("./agent.js");
const { randomUUID } = await import("crypto");

const app = express();
app.use(express.json());

app.post("/agent", async (req, res) => {
  const { message, sessionId } = req.body;
  const sid = sessionId || randomUUID();
  const useRealLLM = process.env.USE_REAL_LLM === "true";

  try {
    const result = await runConversationTurn(sid, message, 1, useRealLLM);
    res.json({ answer: result.finalAnswer, sessionId: sid });
  } catch (err) {
    console.error("[server] error:", err);
    res.status(500).json({ error: String(err) });
  }
});

app.get("/health", (_req, res) => {
  res.json({ status: "ok" });
});

const PORT = process.env.PORT || 3000;
app.listen(PORT, () => {
  console.log(`[server] Service B (TS) listening on :${PORT}`);
});

process.on("SIGTERM", async () => {
  await shutdownTracing();
  process.exit(0);
});
