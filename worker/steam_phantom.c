#define _GNU_SOURCE
#include <dlfcn.h>
#include <libgen.h>
#include <limits.h>
#include <signal.h>
#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/prctl.h>
#include <unistd.h>

static volatile sig_atomic_t running = 1;

static void handle_signal(int sig) {
    (void)sig;
    running = 0;
}

int main(int argc, char *argv[]) {
    if (argc < 2) {
        fprintf(stderr, "Usage: %s <steam_appid>\n", argv[0]);
        return 1;
    }

    const char *appid = argv[1];

    /* Die automatically if the parent wrapper process exits */
    prctl(PR_SET_PDEATHSIG, SIGTERM);

    struct sigaction sa;
    memset(&sa, 0, sizeof(sa));
    sa.sa_handler = handle_signal;
    sigaction(SIGTERM, &sa, NULL);
    sigaction(SIGINT, &sa, NULL);

    /* Override SteamAppId / SteamGameId before loading libsteam_api.so */
    setenv("SteamAppId", appid, 1);
    setenv("SteamGameId", appid, 1);

    /* Locate libsteam_api.so next to this executable */
    char exe_path[PATH_MAX];
    ssize_t len = readlink("/proc/self/exe", exe_path, sizeof(exe_path) - 1);
    if (len <= 0) {
        perror("readlink");
        return 1;
    }
    exe_path[len] = '\0';
    char *dir = dirname(exe_path);

    char so_path[PATH_MAX];
    snprintf(so_path, sizeof(so_path), "%s/libsteam_api.so", dir);

    void *lib = dlopen(so_path, RTLD_NOW);
    if (!lib) {
        fprintf(stderr, "dlopen(%s) failed: %s\n", so_path, dlerror());
        return 1;
    }

    typedef bool (*init_fn)(void);
    typedef void (*run_callbacks_fn)(void);
    typedef void (*shutdown_fn)(void);

    init_fn steam_init = (init_fn)dlsym(lib, "SteamAPI_Init");
    run_callbacks_fn steam_callbacks = (run_callbacks_fn)dlsym(lib, "SteamAPI_RunCallbacks");
    shutdown_fn steam_shutdown = (shutdown_fn)dlsym(lib, "SteamAPI_Shutdown");

    if (!steam_init || !steam_callbacks || !steam_shutdown) {
        fprintf(stderr, "Failed to resolve SteamAPI symbols\n");
        dlclose(lib);
        return 1;
    }

    if (!steam_init()) {
        fprintf(stderr, "SteamAPI_Init() failed for AppID %s (is Steam running and game owned?)\n", appid);
        dlclose(lib);
        return 1;
    }

    fprintf(stderr, "Phantom running for AppID %s\n", appid);

    while (running) {
        steam_callbacks();
        usleep(500000);
    }

    steam_shutdown();
    dlclose(lib);
    return 0;
}

