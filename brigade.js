const { events, Job, Group } = require("brigadier");

events.on("exec", function (e, project) {
    job = conform(e, project)
    job.run().then(result => {
        console.log(result.toString())
    })
})

events.on("push", function (e, project) {
    job = conform(e, project)
    job.run().then(result => {
        console.log(result.toString())
    })
})

events.on("pull_request", function (e, project) {
    start = notify("pending", `Build ${e.buildID} started`, e, project)
    job = conform(e, project)
    Group.runAll([start, job])
        .then(() => {
            return notify("success", `Build ${e.buildID} passed`, e, project).run()
        }).catch(err => {
            return notify("failure", `Build ${e.buildID} failed`, e, project).run()
        });
})

function conform(e, project) {
    var job = new Job("talos", "autonomy/conform:latest")

    job.env = {
        // "DOCKER_HOST": "tcp://docker:2375"
        "DOCKER_USERNAME": project.secrets.DOCKER_USERNAME,
        "DOCKER_PASSWORD": project.secrets.DOCKER_PASSWORD,
    }

    job.tasks = [
        "apk --no-cache add docker",
        "cd /src",
        "conform enforce",
        "conform build",
    ]

    job.docker.enabled = true

    // Unit is milliseconds, 14400000ms = 4h.
    job.timeout = 14400000

    job.host.nodeSelector.set("node-role.kubernetes.io/ci", "")

    job.resourceRequests.memory = "8Gi";
    job.resourceRequests.cpu = "4";
    job.resourceLimits.memory = "16Gi";
    job.resourceLimits.cpu = "8";

    return job
}

function notify(state, msg, e, project) {
    const gh = new Job(`notify-${state}`, "technosophos/github-notify:latest")
    gh.env = {
        GH_REPO: project.repo.name,
        GH_STATE: state,
        GH_DESCRIPTION: msg,
        GH_CONTEXT: "brigade",
        GH_TOKEN: project.secrets.GH_TOKEN,
        GH_COMMIT: e.revision.commit,
        GH_TARGET_URL: `https://ci.dev.autonomy.io/builds/${e.buildID}`,
    }
    return gh
}
