const stateEl = document.querySelector("#backend-state");
const outputEl = document.querySelector("#api-output");
const form = document.querySelector("#create-run");
const revisionStateEl = document.querySelector("#revision-state");
const jobStateEl = document.querySelector("#job-state");
const buttons = {
  loadDemo: document.querySelector("#load-demo"),
  freeze: document.querySelector("#freeze"),
  start: document.querySelector("#start"),
  finalize: document.querySelector("#finalize"),
  download: document.querySelector("#download")
};

let revisionId = "";
let version = 0;

async function refreshHealth() {
  const res = await fetch("/readyz");
  const data = await res.json();
  stateEl.textContent = `后端状态：${data.status}`;
}

function show(data) {
  outputEl.textContent = JSON.stringify(data, null, 2);
  const revision = data.revision || data;
  if (revision.revision_id) {
    revisionId = revision.revision_id;
    version = revision.version;
    revisionStateEl.textContent = `修订 ${revisionId} / ${revision.state} / v${version}`;
  }
}

async function post(path, body) {
  const res = await fetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json", "Idempotency-Key": `${path}-${Date.now()}` },
    body: JSON.stringify(body)
  });
  const data = await res.json();
  show(data);
  if (!res.ok) {
    throw new Error(data.message || "request failed");
  }
  return data;
}

form.addEventListener("submit", async (event) => {
  event.preventDefault();
  const formData = new FormData(form);
  const body = {
    device_id: formData.get("device_id"),
    business_key: formData.get("business_key")
  };
  const res = await fetch("/api/v1/sterilization-runs", {
    method: "POST",
    headers: { "Content-Type": "application/json", "Idempotency-Key": body.business_key },
    body: JSON.stringify(body)
  });
  show(await res.json());
});

buttons.loadDemo.addEventListener("click", async () => {
  if (!revisionId) return;
  const probes = [
    { probe_id: "load-1", role: "load", required: true, unit: "C" },
    { probe_id: "chamber-1", role: "chamber", required: true, unit: "C" },
    { probe_id: "pressure-1", role: "pressure", required: true, unit: "kPa_abs" }
  ];
  await post(`/api/v1/revisions/${revisionId}/probes`, { expected_version: version, probes });
  const calibrations = [
    { probe_id: "load-1", valid_from_nanos: 0, valid_until_nanos: 120000000000, offset_c: 0, uncertainty_c: 0 },
    { probe_id: "chamber-1", valid_from_nanos: 0, valid_until_nanos: 120000000000, offset_c: 0, uncertainty_c: 0 }
  ];
  await post(`/api/v1/revisions/${revisionId}/calibrations`, { expected_version: version, calibrations });
  const requirements = {
    run_start_nanos: 0, run_end_nanos: 120000000000, sample_step_nanos: 30000000000, max_gap_nanos: 60000000000,
    exposure_min_c: 121, exposure_min_nanos: 60000000000, confirm_nanos: 30000000000, grace_nanos: 30000000000,
    spread_max_c: 2, min_lethality_minutes: 1, steam_tolerance_c: 3, steam_allowed_nanos: 30000000000,
    t_ref_c: 121, z_c: 10, steam_table_version: "ui-demo",
    required_probe_ids: ["load-1"],
    frozen_steam_table: [{ pressure_kpa: 198.67, saturated_c: 120 }, { pressure_kpa: 205, saturated_c: 121 }, { pressure_kpa: 211, saturated_c: 122 }]
  };
  await post(`/api/v1/revisions/${revisionId}/requirements`, { expected_version: version, requirements });
  const samples = [0, 60000000000, 120000000000].flatMap((at) => [
    { probe_id: "load-1", at_nanos: at, kind: "temperature", value: 122, unit: "C" },
    { probe_id: "chamber-1", at_nanos: at, kind: "temperature", value: 121, unit: "C" },
    { probe_id: "pressure-1", at_nanos: at, kind: "pressure", value: 205, unit: "kPa_abs" }
  ]);
  await post(`/api/v1/revisions/${revisionId}/samples:batch`, { expected_version: version, samples });
});

buttons.freeze.addEventListener("click", () => revisionId && post(`/api/v1/revisions/${revisionId}:freeze`, { expected_version: version }));
buttons.start.addEventListener("click", async () => {
  if (!revisionId) return;
  const data = await post(`/api/v1/revisions/${revisionId}:start`, { expected_version: version });
  jobStateEl.textContent = `任务 ${data.job_id} 已完成`;
});
buttons.finalize.addEventListener("click", () => revisionId && post(`/api/v1/revisions/${revisionId}:finalize`, { expected_version: version }));
buttons.download.addEventListener("click", () => {
  if (revisionId) window.location.href = `/api/v1/revisions/${revisionId}/evidence`;
});

refreshHealth().catch((err) => {
  stateEl.textContent = `后端状态读取失败：${err.message}`;
});
