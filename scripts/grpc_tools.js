#!/usr/bin/env node
/* eslint-disable no-console */
const { execFileSync, spawn } = require("node:child_process");
const { randomUUID } = require("node:crypto");
const fs = require("node:fs");

const args = process.argv.slice(2);
const mode = args[0] || "smoke";

const target = process.env.GRPC_TARGET || "localhost:50051";
const userId = process.env.USER_ID || randomUUID();

function runGrpcurl(args, input) {
  try {
    return execFileSync("grpcurl", args, { input, encoding: "utf8" }).trim();
  } catch (err) {
    const stderr = err.stderr ? err.stderr.toString() : "";
    const stdout = err.stdout ? err.stdout.toString() : "";
    console.error(stderr || stdout || err.message);
    process.exit(1);
  }
}

function getTasksWithProgress() {
  const resp = runGrpcurl([
    "-plaintext",
    "-d",
    JSON.stringify({ userId }),
    target,
    "tasks.v1.TaskService/GetTasksWithProgress",
  ]);
  return JSON.parse(resp || "{}");
}

function getTask(taskId) {
  runGrpcurl([
    "-plaintext",
    "-d",
    JSON.stringify({ taskId }),
    target,
    "tasks.v1.TaskService/GetTask",
  ]);
}

function processEvent(eventId, taskId, amount) {
  runGrpcurl([
    "-plaintext",
    "-d",
    JSON.stringify({
      event: {
        eventId,
        userId,
        roomId: "room-1",
        type: "progress_update",
        payload: { taskId, amount },
        createdAt: new Date().toISOString(),
      },
    }),
    target,
    "tasks.v1.TaskService/ProcessEvent",
  ]);
}

function streamEventsBatch(taskId, amount) {
  const batchPayload = JSON.stringify({
    events: [
      {
        eventId: randomUUID(),
        userId,
        roomId: "room-1",
        type: "progress_update",
        payload: { taskId, amount },
        createdAt: new Date().toISOString(),
      },
    ],
  });
  return execFileSync(
    "grpcurl",
    ["-plaintext", "-d", "@", target, "tasks.v1.TaskService/StreamEvents"],
    { input: batchPayload + "\n", encoding: "utf8" }
  ).trim();
}

function claimReward(taskId) {
  runGrpcurl([
    "-plaintext",
    "-d",
    JSON.stringify({ userId, taskId }),
    target,
    "tasks.v1.TaskService/ClaimReward",
  ]);
}

function pickTaskId() {
  const list = getTasksWithProgress();
  const task = (list.tasks || []).find((t) => t.target > 1) || (list.tasks || [])[0];
  if (!task) {
    console.error("No tasks found.");
    process.exit(1);
  }
  return task.id;
}

function smoke() {
  console.log(`Target: ${target}`);
  console.log("-> list services");
  runGrpcurl(["-plaintext", target, "list"]);

  console.log("-> GetTasksWithProgress");
  const list = getTasksWithProgress();
  const task = (list.tasks || [])[0];
  if (!task || !task.id) {
    console.error("No tasks found to run smoke test.");
    process.exit(1);
  }
  const taskId = task.id;

  console.log(`User ID: ${userId}`);
  console.log(`Task ID: ${taskId}`);

  console.log("-> GetTask");
  getTask(taskId);

  console.log("-> ProcessEvent");
  processEvent(randomUUID(), taskId, 1);

  console.log("-> ClaimReward");
  claimReward(taskId);

  console.log("-> GetTasksWithProgress (final)");
  const finalResp = getTasksWithProgress();
  console.log("Final progress:");
  console.log(JSON.stringify(finalResp, null, 2));

  console.log("-> StreamEvents (batch)");
  const streamEventsResp = streamEventsBatch(taskId, 1);
  console.log("StreamEvents response:");
  console.log(streamEventsResp);

  console.log("-> SubscribeProgress (first message)");
  const subscribeResp = execFileSync(
    "grpcurl",
    ["-plaintext", "-d", JSON.stringify({ userId }), target, "tasks.v1.TaskService/SubscribeProgress"],
    { encoding: "utf8" }
  ).trim();
  console.log("SubscribeProgress first message:");
  console.log(subscribeResp);
}

function reliability() {
  const taskId = process.env.TASK_ID || pickTaskId();
  const list = getTasksWithProgress();
  const task = (list.tasks || []).find((t) => t.id === taskId);
  const targetValue = task ? task.target || 1 : 1;

  console.log(`Target: ${target}`);
  console.log(`User ID: ${userId}`);
  console.log(`Task ID: ${taskId} (target=${targetValue})`);

  if (targetValue <= 1) {
    console.warn("Task target is 1; idempotency check may be inconclusive.");
  }

  console.log("-> Idempotency: ProcessEvent duplicate");
  const eventId = randomUUID();
  const before = getTasksWithProgress().progress?.find((p) => p.taskId === taskId)?.progress || 0;
  processEvent(eventId, taskId, 1);
  const afterFirst = getTasksWithProgress().progress?.find((p) => p.taskId === taskId)?.progress || 0;
  processEvent(eventId, taskId, 1);
  const afterSecond = getTasksWithProgress().progress?.find((p) => p.taskId === taskId)?.progress || 0;
  console.log(`Progress: before=${before}, after1=${afterFirst}, after2=${afterSecond}`);
  if (targetValue > 1 && afterSecond > afterFirst) {
    console.error("Idempotency failed: duplicate ProcessEvent increased progress.");
    process.exit(1);
  }

  console.log("-> Idempotency: StreamEvents duplicate");
  const streamEventId = randomUUID();
  const beforeStream = getTasksWithProgress().progress?.find((p) => p.taskId === taskId)?.progress || 0;
  const streamBatch = JSON.stringify({
    events: [
      {
        eventId: streamEventId,
        userId,
        roomId: "room-1",
        type: "progress_update",
        payload: { taskId, amount: 1 },
        createdAt: new Date().toISOString(),
      },
    ],
  });
  runGrpcurl(["-plaintext", "-d", streamBatch, target, "tasks.v1.TaskService/StreamEvents"]);
  const afterStreamFirst = getTasksWithProgress().progress?.find((p) => p.taskId === taskId)?.progress || 0;
  runGrpcurl(["-plaintext", "-d", streamBatch, target, "tasks.v1.TaskService/StreamEvents"]);
  const afterStreamSecond = getTasksWithProgress().progress?.find((p) => p.taskId === taskId)?.progress || 0;
  console.log(`Progress: before=${beforeStream}, after1=${afterStreamFirst}, after2=${afterStreamSecond}`);
  if (targetValue > 1 && afterStreamSecond > afterStreamFirst) {
    console.error("Idempotency failed: duplicate StreamEvents increased progress.");
    process.exit(1);
  }

  console.log("-> Idempotency: ClaimReward twice");
  processEvent(randomUUID(), taskId, targetValue);
  claimReward(taskId);
  claimReward(taskId);
  console.log("ClaimReward duplicate: OK");
}

function stress() {
  const durationSec = Number(process.env.DURATION_SEC || 0);
  const concurrency = Number(process.env.CONCURRENCY || 32);
  const batchSize = Number(process.env.BATCH_SIZE || 50);
  const taskId = process.env.TASK_ID || pickTaskId();
  const errorLogPath = process.env.ERROR_LOG || "scripts/grpc_errors.txt";

  const endAt = durationSec > 0 ? Date.now() + durationSec * 1000 : Infinity;
  let inFlight = 0;
  let sent = 0;
  let ok = 0;
  let failed = 0;
  let stopping = false;

  function printStats() {
    console.log(`Target: ${target}`);
    console.log(`Duration: ${durationSec === 0 ? "infinite" : `${durationSec}s`}`);
    console.log(`Concurrency: ${concurrency}`);
    console.log(`Batch size: ${batchSize}`);
    console.log(`User ID: ${userId}`);
    console.log(`Task ID: ${taskId}`);
    console.log(`Sent batches: ${sent}`);
    console.log(`OK: ${ok}`);
    console.log(`Failed: ${failed}`);
  }

  function buildBatch() {
    const events = [];
    for (let i = 0; i < batchSize; i += 1) {
      events.push({
        eventId: randomUUID(),
        userId,
        roomId: "room-1",
        type: "progress_update",
        payload: { taskId, amount: 1 },
        createdAt: new Date().toISOString(),
      });
    }
    return JSON.stringify({ events });
  }

  function runStreamEvents() {
    return new Promise((resolve) => {
      const child = spawn("grpcurl", ["-plaintext", "-d", "@", target, "tasks.v1.TaskService/StreamEvents"]);
      let stderr = "";

      child.stderr.on("data", (chunk) => {
        stderr += chunk.toString();
      });
      child.on("close", (code, signal) => {
        if (code === 0 && stderr.trim() === "") {
          ok += 1;
        } else {
          failed += 1;
          const line = `[${new Date().toISOString()}] code=${code} signal=${signal || ""} stderr=${stderr.trim()}\n`;
          fs.appendFileSync(errorLogPath, line, "utf8");
        }
        resolve();
      });

      child.stdin.write(buildBatch() + "\n");
      child.stdin.end();
    });
  }

  function tick() {
    while (inFlight < concurrency && Date.now() < endAt) {
      inFlight += 1;
      sent += 1;
      runStreamEvents().finally(() => {
        inFlight -= 1;
      });
    }

    if (Date.now() >= endAt && inFlight === 0) {
      printStats();
      process.exit(0);
    }

    setTimeout(tick, 5);
  }

  process.on("SIGINT", () => {
    if (stopping) {
      process.exit(130);
    }
    stopping = true;
    console.log("\nReceived SIGINT, waiting for in-flight batches to finish...");
    const wait = setInterval(() => {
      if (inFlight === 0) {
        clearInterval(wait);
        printStats();
        process.exit(0);
      }
    }, 50);
  });

  tick();
}

switch (mode) {
  case "smoke":
    smoke();
    break;
  case "reliability":
    reliability();
    break;
  case "stress":
    stress();
    break;
  default:
    console.log("Usage: node scripts/grpc_tools.js <smoke|reliability|stress>");
    process.exit(1);
}
