import assert from "node:assert/strict";
import test from "node:test";
import type { WorkerRequest, WorkerResponse } from "../shared/types";

test("worker sends fresh intent and always requests current daemon scoring", async () => {
  let listener: (request: WorkerRequest, sender: unknown, reply: (response: WorkerResponse) => void) => void;
  const urls: string[]=[];
  const originalFetch=globalThis.fetch;
  const originalChrome=globalThis.chrome;
  globalThis.chrome={
    action:{onClicked:{addListener:()=>{}}},
    runtime:{onMessage:{addListener:(fn:typeof listener)=>{listener=fn;}}},
    storage:{local:{get:(_keys: unknown, callback:(settings:unknown)=>void)=>callback({token:"test-token",daemonUrl:"http://127.0.0.1:8765"})}}
  } as unknown as typeof chrome;
  globalThis.fetch=async (url)=>{
    urls.push(String(url));
    return new Response(JSON.stringify({risk_score:0,not_evaluated:[],trace:[]}),{status:200});
  };
  try {
    await import("../background/worker");
    const listing={marketplace:"craigslist",listing_url:"https://sfbay.craigslist.org/apa/1.html",title:"Studio",contact:{}};
    const send=(force:boolean)=>new Promise<WorkerResponse>(resolve=>listener({type:"ANALYZE_LISTING",listing,force},{},resolve));
    await send(false); await send(false); await send(true);
    assert.deepEqual(urls,["http://127.0.0.1:8765/api/analyze","http://127.0.0.1:8765/api/analyze","http://127.0.0.1:8765/api/analyze?fresh=true"]);
  } finally {globalThis.fetch=originalFetch;globalThis.chrome=originalChrome;}
});
