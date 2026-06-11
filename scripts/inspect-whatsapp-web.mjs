import { mkdir, writeFile } from "node:fs/promises";
import { chromium } from "playwright";

const TARGET_URL = "https://web.whatsapp.com/";
const MAX_REQUESTS = 250;

function simplifyHeaders(headers) {
  const keep = ["content-type", "cache-control", "content-length", "etag"];
  return Object.fromEntries(
    Object.entries(headers).filter(([key]) => keep.includes(key.toLowerCase())),
  );
}

function trimRequest(request) {
  return {
    method: request.method(),
    resourceType: request.resourceType(),
    url: request.url(),
  };
}

function trimResponse(response) {
  return {
    status: response.status(),
    url: response.url(),
    headers: simplifyHeaders(response.headers()),
  };
}

const browser = await chromium.launch({ headless: true });
const context = await browser.newContext();
const page = await context.newPage();

const requests = [];
const responses = [];
const consoleMessages = [];
const pageErrors = [];

page.on("request", (request) => {
  if (requests.length < MAX_REQUESTS) requests.push(trimRequest(request));
});

page.on("response", (response) => {
  if (responses.length < MAX_REQUESTS) responses.push(trimResponse(response));
});

page.on("console", (message) => {
  consoleMessages.push({
    type: message.type(),
    text: message.text(),
  });
});

page.on("pageerror", (error) => {
  pageErrors.push(error.message);
});

await page.goto(TARGET_URL, { waitUntil: "domcontentloaded", timeout: 120000 });
await page.waitForTimeout(8000);

const snapshot = await page.evaluate(async () => {
  const localStorageEntries = Object.entries(localStorage);
  const sessionStorageEntries = Object.entries(sessionStorage);
  const serviceWorkerRegistrations = "serviceWorker" in navigator
    ? await navigator.serviceWorker.getRegistrations()
    : [];

  const databases = indexedDB.databases ? await indexedDB.databases() : [];

  return {
    location: window.location.href,
    title: document.title,
    localStorageKeys: localStorageEntries.map(([key]) => key),
    sessionStorageKeys: sessionStorageEntries.map(([key]) => key),
    indexedDbDatabases: databases,
    serviceWorkers: serviceWorkerRegistrations.map((registration) => ({
      scope: registration.scope,
      activeScriptURL: registration.active?.scriptURL ?? null,
      installingScriptURL: registration.installing?.scriptURL ?? null,
      waitingScriptURL: registration.waiting?.scriptURL ?? null,
    })),
    scripts: Array.from(document.scripts).map((script) => script.src).filter(Boolean),
    styleSheets: Array.from(document.styleSheets)
      .map((sheet) => sheet.href)
      .filter(Boolean),
    domMarkers: {
      rootIds: Array.from(document.querySelectorAll("[id]"))
        .map((node) => node.id)
        .filter(Boolean)
        .slice(0, 50),
      dialogCount: document.querySelectorAll('[role="dialog"]').length,
      buttonCount: document.querySelectorAll("button").length,
    },
  };
});

const report = {
  capturedAt: new Date().toISOString(),
  target: TARGET_URL,
  snapshot,
  requests,
  responses,
  consoleMessages,
  pageErrors,
};

const reportFileName = report.capturedAt.replaceAll(":", "-") + ".json";
await mkdir(new URL("../reports/", import.meta.url), { recursive: true });
await writeFile(
  new URL(`../reports/${reportFileName}`, import.meta.url),
  JSON.stringify(report, null, 2) + "\n",
  "utf8",
);

console.log(JSON.stringify(report, null, 2));

await browser.close();
