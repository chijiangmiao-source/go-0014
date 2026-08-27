package store

var schemaStatements = []string{
	`create table if not exists sterilization_runs (
		id integer primary key autoincrement,
		device_id text not null,
		business_key text not null unique,
		current_revision_id text,
		created_at_nanos integer not null
	)`,
	`create table if not exists revisions (
		id text primary key,
		run_id integer not null references sterilization_runs(id),
		state text not null,
		lock_version integer not null,
		inputs_json blob,
		snapshot_json blob,
		snapshot_hash text,
		algorithm_version text,
		conclusion text,
		result_json blob,
		predecessor_id text,
		sealed integer not null default 0
	)`,
	`create table if not exists state_events (
		revision_id text not null references revisions(id),
		ordinal integer not null,
		from_state text not null,
		to_state text not null,
		reason_code text not null,
		result_version integer not null,
		created_at_nanos integer not null,
		primary key(revision_id, ordinal)
	)`,
	`create table if not exists analysis_jobs (
		id text primary key,
		revision_id text not null references revisions(id),
		type text not null,
		status text not null,
		job_json blob not null,
		created_at_nanos integer not null default 0
	)`,
	`create unique index if not exists analysis_jobs_one_active
		on analysis_jobs(revision_id, type)
		where status in ('QUEUED', 'RUNNING')`,
	`create table if not exists exposure_segments (
		revision_id text primary key references revisions(id),
		start_nanos integer not null,
		end_nanos integer not null,
		duration_nanos integer not null,
		cold_point_c real not null
	)`,
	`create table if not exists probe_metrics (
		revision_id text not null references revisions(id),
		probe_id text not null,
		minimum_c real not null,
		maximum_c real not null,
		equivalent_minutes real not null,
		summary_hash text not null,
		primary key(revision_id, probe_id)
	)`,
	`create table if not exists findings (
		revision_id text not null references revisions(id),
		ordinal integer not null,
		rule_code text not null,
		probe_or_area text,
		start_nanos integer,
		end_nanos integer,
		first_failed_nanos integer,
		measured real not null,
		threshold_value real not null,
		margin real not null,
		unit text not null,
		severity text not null,
		primary key(revision_id, ordinal)
	)`,
	`create table if not exists evidence_packages (
		revision_id text primary key references revisions(id),
		bytes blob not null,
		sha256 text not null,
		length integer not null,
		complete integer not null,
		version integer not null
	)`,
	`create table if not exists replay_results (
		id text primary key,
		revision_id text not null references revisions(id),
		replay_json blob not null
	)`,
	`create trigger if not exists revisions_sealed_no_inputs
		before update of inputs_json, snapshot_json on revisions
		when old.state in ('SEALED_PASS', 'SEALED_FAIL')
		begin
			select raise(abort, 'sealed revision is immutable');
		end`,
}
