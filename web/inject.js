(function() {
	["__questTracker", "__questTrackerV2", "__questTrackerV3", "__questTrackerV4", "__questTrackerV5", "__questTrackerV6", "__questTrackerV7", "__questTrackerV8"].forEach((key) => {
		try {
			if (window[key] && typeof window[key].stop === "function") {
				window[key].stop();
			}
			delete window[key];
		} catch (e) {}
	});

	["__questTrackerV6Timer", "__questTrackerV7Timer", "__questTrackerV8Timer"].forEach((tKey) => {
		if (window[tKey]) {
			clearInterval(window[tKey]);
			delete window[tKey];
		}
	});

	if (window.__questTrackerV9) {
		return JSON.stringify(window.__questTrackerV9.getState());
	}

	delete window.$;
	let wpRequire = webpackChunkdiscord_app.push([[Symbol()], {}, r => r]);
	webpackChunkdiscord_app.pop();

	const modules = Object.values(wpRequire.c);

	const findByKeys = (predicate) => {
		for (const m of modules) {
			const exp = m?.exports;
			if (!exp || typeof exp !== "object") continue;
			for (const k of ["A", "Ay", "Bo", "h", "Z", "default"]) {
				try {
					const val = exp[k];
					if (val && predicate(val)) return val;
				} catch (e) {}
			}
		}
		return null;
	};

	const ApplicationStreamingStore =
		modules.find(x => x?.exports?.A?.__proto__?.getStreamerActiveStreamMetadata)?.exports?.A ||
		findByKeys(v => v?.__proto__?.getStreamerActiveStreamMetadata);

	const RunningGameStore =
		modules.find(x => x?.exports?.Ay?.getRunningGames)?.exports?.Ay ||
		findByKeys(v => typeof v?.getRunningGames === "function" && typeof v?.getGameForPID === "function");

	const QuestsStore =
		modules.find(x => x?.exports?.A?.__proto__?.getQuest)?.exports?.A ||
		findByKeys(v => v?.__proto__?.getQuest);

	const ChannelStore =
		modules.find(x => x?.exports?.A?.__proto__?.getAllThreadsForParent)?.exports?.A ||
		findByKeys(v => v?.__proto__?.getAllThreadsForParent);

	const GuildChannelStore =
		modules.find(x => x?.exports?.A?.__proto__?.getSFWDefaultChannel)?.exports?.A ||
		findByKeys(v => v?.__proto__?.getSFWDefaultChannel);

	const FluxDispatcher =
		modules.find(x => x?.exports?.h?.__proto__?.flushWaitQueue)?.exports?.h ||
		findByKeys(v => v?.__proto__?.flushWaitQueue);

	const api =
		modules.find(x => x?.exports?.Bo?.get && x?.exports?.Bo?.post)?.exports?.Bo ||
		findByKeys(v => typeof v?.get === "function" && typeof v?.post === "function" && typeof v?.getAPIBaseURL === "function");

	const UserStore =
		modules.find(x => x?.exports?.default?.__proto__?.getCurrentUser)?.exports?.default ||
		findByKeys(v => v?.__proto__?.getCurrentUser);

	const ApplicationStore =
		modules.find(x => x?.exports?.A?.__proto__?.getApplication)?.exports?.A ||
		findByKeys(v => v?.__proto__?.getApplication && v?.__proto__?.getApplicationByName);

	const VirtualCurrencyStore =
		modules.find(x => x?.exports?.A?.getName?.() === "VirtualCurrencyStore")?.exports?.A ||
		findByKeys(v => v?.getName?.() === "VirtualCurrencyStore" || typeof v?.fetchBalance === "function");

	const supportedTasks = [
		"WATCH_VIDEO",
		"PLAY_ON_DESKTOP",
		"STREAM_ON_DESKTOP",
		"PLAY_ACTIVITY",
		"WATCH_VIDEO_ON_MOBILE"
	];

	const state = {
		isRunning: false,
		autoEnroll: true,
		autoStart: false,
		currentQuestId: null,
		enrollingIds: new Set(),
		failedEnrollIds: new Set(),
		liveProgress: {},
		orbBalance: null,
		initialOrbBalance: null,
		lastBalanceFetch: 0,
		isTicking: false,
		logs: [],
		cleanupFns: [],
		abort: false,
		runToken: 0,
		initError: (!QuestsStore || !api || !FluxDispatcher) ? "Не найдены внутренние модули Discord" : null
	};

	function addLog(msg, level = "info") {
		const now = new Date().toLocaleTimeString("ru-RU");
		state.logs.unshift({ time: now, text: msg, level });
		if (state.logs.length > 80) state.logs.length = 80;
	}

	function formatErr(e) {
		if (!e) return "неизвестная ошибка";
		if (e.body && e.body.message) return e.body.message + (e.status ? " (HTTP " + e.status + ")" : "");
		if (e.message) return e.message;
		return String(e);
	}

	function applyBalance(val) {
		if (typeof val !== "number" || Number.isNaN(val)) return;
		if (state.initialOrbBalance === null) {
			state.initialOrbBalance = val;
		}
		state.orbBalance = val;
	}

	function readStoreBalance() {
		try {
			if (VirtualCurrencyStore) {
				const val = typeof VirtualCurrencyStore.getBalance === "function"
					? VirtualCurrencyStore.getBalance()
					: (VirtualCurrencyStore.balance ?? VirtualCurrencyStore._balance);
				applyBalance(val);
			}
		} catch (e) {}
	}

	async function refreshBalance(force) {
		readStoreBalance();
		if (!api) return;

		const now = Date.now();
		if (!force && state.orbBalance !== null && now - state.lastBalanceFetch < 15000) return;
		state.lastBalanceFetch = now;

		try {
			const res = await api.get({ url: "/users/@me/virtual-currency/balance" });
			const body = res?.body;
			if (body) {
				const num = body.balance ?? body.orb_balance ?? body.virtual_currency_balance;
				applyBalance(num);
			}
			readStoreBalance();
		} catch (e) {
			readStoreBalance();
		}
	}

	function getUserInfo() {
		try {
			const u = UserStore?.getCurrentUser?.();
			if (!u) return null;
			const avatarUrl = u.avatar
				? "https://cdn.discordapp.com/avatars/" + u.id + "/" + u.avatar + ".png?size=64"
				: "https://cdn.discordapp.com/embed/avatars/0.png";
			return {
				id: u.id,
				username: u.username,
				globalName: u.globalName || u.username,
				avatarUrl: avatarUrl
			};
		} catch (e) {
			return null;
		}
	}

	function extractQuestImages(q) {
		const urls = [];
		const pushAsset = (val) => {
			if (!val || typeof val !== "string") return;
			if (val.startsWith("http")) {
				urls.push(val);
				return;
			}
			const clean = val.replace(/^\//, "");
			if (clean.startsWith("quests/") || clean.startsWith("app-icons/") || clean.startsWith("app-assets/")) {
				urls.push("https://cdn.discordapp.com/" + clean);
			} else if (clean.startsWith(q.id + "/")) {
				urls.push("https://cdn.discordapp.com/quests/" + clean);
			} else {
				urls.push("https://cdn.discordapp.com/quests/" + q.id + "/" + clean);
				urls.push("https://cdn.discordapp.com/" + clean);
				if (!clean.includes(".")) {
					urls.push("https://cdn.discordapp.com/quests/" + q.id + "/" + clean + ".png");
					urls.push("https://cdn.discordapp.com/quests/" + q.id + "/" + clean + ".webp");
				}
			}
		};

		const assets = q.config?.assets || {};
		pushAsset(assets.gameTile);
		pushAsset(assets.logotype);

		const appId = q.config?.application?.id;
		const appIcon = q.config?.application?.icon;
		if (appId) {
			try {
				const appObj = ApplicationStore?.getApplication?.(appId);
				if (appObj && typeof appObj.getIconURL === "function") {
					const iconUrl = appObj.getIconURL(128);
					if (iconUrl) urls.push(iconUrl);
				}
			} catch (e) {}
			if (appIcon) {
				urls.push("https://cdn.discordapp.com/app-icons/" + appId + "/" + appIcon + ".webp?size=128");
				urls.push("https://cdn.discordapp.com/app-icons/" + appId + "/" + appIcon + ".png?size=128");
			}
		}

		pushAsset(assets.questBarHero);
		pushAsset(assets.hero);

		const rewards = q.config?.rewardsConfig?.rewards || q.config?.rewards || [];
		for (const r of rewards) {
			if (r.asset) pushAsset(r.asset);
		}

		return [...new Set(urls)];
	}

	function extractRewardInfo(q) {
		const rewards = q.config?.rewardsConfig?.rewards || q.config?.rewards || [];
		let orbAmount = 0;
		let rewardName = "";
		let rewardType = "Предмет";

		for (const r of rewards) {
			const name = r.messages?.name || r.messages?.nameWithArticle || r.name || "";
			if (r.type === 4 || (typeof r.orbQuantity === "number" && r.orbQuantity > 0)) {
				orbAmount += (r.orbQuantity || 0);
				rewardType = "Orbs";
				if (!rewardName) rewardName = name || (orbAmount + " Discord Orbs");
			} else {
				if (!rewardName && name) rewardName = name;
				if (r.type === 3) rewardType = "Украшение";
				else if (r.type === 5) rewardType = "Nitro";
				else if (r.type === 1 || r.type === 2) rewardType = "В игру";
			}
		}

		if (!rewardName) {
			rewardName = orbAmount > 0 ? (orbAmount + " Orbs") : "Награда квеста";
		}

		return {
			name: rewardName,
			type: rewardType,
			orbs: orbAmount,
			claimed: Boolean(q.userStatus?.claimedAt)
		};
	}

	function getTaskInfo(q) {
		const taskConfig = q.config?.taskConfig ?? q.config?.taskConfigV2;
		if (!taskConfig || !taskConfig.tasks) return null;
		const taskName = supportedTasks.find(t => taskConfig.tasks[t] != null) || Object.keys(taskConfig.tasks)[0] || "UNKNOWN";
		const taskData = taskConfig.tasks[taskName] || { target: 0 };
		return { taskConfig, taskName, taskData };
	}

	function parseQuests() {
		if (!QuestsStore || !QuestsStore.quests) return [];
		const list = [];
		for (const q of QuestsStore.quests.values()) {
			const info = getTaskInfo(q);
			if (!info) continue;
			const { taskName, taskData } = info;
			const secondsNeeded = taskData.target || 0;
			const storeDone = Math.floor(q.userStatus?.progress?.[taskName]?.value ?? (q.userStatus?.streamProgressSeconds ?? 0));
			const liveDone = state.liveProgress[q.id] || 0;
			const secondsDone = Math.max(storeDone, liveDone);
			const expired = new Date(q.config.expiresAt).getTime() <= Date.now();
			const completed = Boolean(q.userStatus?.completedAt);
			const enrolled = Boolean(q.userStatus?.enrolledAt);

			if (expired && !completed) continue;

			const isExternalAchievement = taskName === "ACHIEVEMENT_IN_ACTIVITY" || taskName === "ACHIEVEMENT_IN_GAME";

			list.push({
				id: q.id,
				name: q.config.messages?.questName || "Без названия",
				game: q.config.application?.name || q.config.messages?.gameTitle || "Discord Quest",
				imageUrls: extractQuestImages(q),
				taskType: taskName,
				isStepTask: isExternalAchievement,
				isExternal: isExternalAchievement,
				supported: supportedTasks.includes(taskName),
				enrolled: enrolled,
				completed: completed,
				enrollFailed: state.failedEnrollIds.has(q.id),
				secondsDone: completed ? secondsNeeded : Math.min(secondsDone, secondsNeeded),
				secondsNeeded: secondsNeeded,
				percent: completed ? 100 : (secondsNeeded > 0 ? Math.min(100, Math.floor((secondsDone / secondsNeeded) * 100)) : 0),
				expiresAt: q.config.expiresAt,
				reward: extractRewardInfo(q)
			});
		}
		return list;
	}

	async function enrollQuest(questId, questName) {
		if (state.enrollingIds.has(questId) || state.failedEnrollIds.has(questId)) return false;
		state.enrollingIds.add(questId);
		try {
			await api.post({
				url: "/quests/" + questId + "/enroll",
				body: { location: 11, is_targeted: false }
			});
			addLog("Принят квест: «" + questName + "»", "ok");
			return true;
		} catch (e) {
			state.failedEnrollIds.add(questId);
			addLog("Не удалось принять «" + questName + "»: " + formatErr(e), "warn");
			return false;
		} finally {
			state.enrollingIds.delete(questId);
		}
	}

	async function waitForEnrollment(questId, timeoutMs = 3000) {
		const started = Date.now();
		while (Date.now() - started < timeoutMs) {
			const q = QuestsStore.quests.get(questId);
			if (q && q.userStatus?.enrolledAt) return q;
			await new Promise(r => setTimeout(r, 250));
		}
		return QuestsStore.quests.get(questId);
	}

	async function enrollAllAvailable() {
		if (!QuestsStore || !QuestsStore.quests || !api) return 0;
		let count = 0;
		for (const q of QuestsStore.quests.values()) {
			const info = getTaskInfo(q);
			if (!info) continue;
			const hasSupported = supportedTasks.includes(info.taskName);
			const expired = new Date(q.config.expiresAt).getTime() <= Date.now();
			const completed = Boolean(q.userStatus?.completedAt);
			const enrolled = Boolean(q.userStatus?.enrolledAt);

			if (!enrolled && !completed && !expired && hasSupported && !state.failedEnrollIds.has(q.id)) {
				const ok = await enrollQuest(q.id, q.config.messages?.questName || q.id);
				if (ok) {
					count++;
					await new Promise(r => setTimeout(r, 450));
				}
			}
		}
		return count;
	}

	function runCleanup() {
		while (state.cleanupFns.length > 0) {
			const fn = state.cleanupFns.pop();
			try { fn(); } catch (e) {}
		}
	}

	function stopInternal(silent) {
		state.runToken++;
		state.abort = true;
		state.isRunning = false;
		state.currentQuestId = null;
		runCleanup();
		if (!silent) {
			addLog("Выполнение остановлено пользователем.", "warn");
		}
	}

	function getTaskPriority(taskName) {
		if (taskName === "WATCH_VIDEO" || taskName === "WATCH_VIDEO_ON_MOBILE") return 1;
		if (taskName === "PLAY_ACTIVITY") return 2;
		if (taskName === "PLAY_ON_DESKTOP") return 3;
		if (taskName === "STREAM_ON_DESKTOP") return 4;
		return 5;
	}

	function collectEligibleQueue() {
		const items = [...QuestsStore.quests.values()].filter(x => {
			const info = getTaskInfo(x);
			return x.userStatus?.enrolledAt &&
				!x.userStatus?.completedAt &&
				new Date(x.config.expiresAt).getTime() > Date.now() &&
				info && supportedTasks.includes(info.taskName);
		});

		items.sort((a, b) => {
			const iA = getTaskInfo(a);
			const iB = getTaskInfo(b);
			const pA = getTaskPriority(iA.taskName);
			const pB = getTaskPriority(iB.taskName);
			if (pA !== pB) return pA - pB;
			return (iA.taskData?.target || 0) - (iB.taskData?.target || 0);
		});
		return items;
	}

	function executeQuest(quest, token) {
		return new Promise((resolve) => {
			if (state.abort || state.runToken !== token || !quest || quest.userStatus?.completedAt) {
				resolve();
				return;
			}

			state.currentQuestId = quest.id;
			const pid = Math.floor(Math.random() * 30000) + 1000;
			const questName = quest.config.messages.questName;
			const { taskName, taskData } = getTaskInfo(quest);
			const applicationId = quest.config.application?.id ?? taskData.applications?.[0]?.id;
			const secondsNeeded = taskData.target;
			let secondsDone = Math.floor(quest.userStatus?.progress?.[taskName]?.value ?? 0);
			state.liveProgress[quest.id] = secondsDone;

			const finish = async () => {
				if (state.runToken === token) {
					await refreshBalance(true);
				}
				resolve();
			};

			if (taskName === "WATCH_VIDEO" || taskName === "WATCH_VIDEO_ON_MOBILE") {
				const speed = 7;
				const enrolledAt = quest.userStatus?.enrolledAt ? new Date(quest.userStatus.enrolledAt).getTime() : Date.now();
				let completed = false;
				addLog("Видео-квест: «" + questName + "» (" + secondsNeeded + " сек)", "run");

				(async () => {
					try {
						while (!state.abort && state.runToken === token) {
							const remaining = Math.min(speed, secondsNeeded - secondsDone);
							await new Promise(r => setTimeout(r, Math.max(1, remaining) * 1000));
							if (state.abort || state.runToken !== token) return resolve();

							const maxAllowed = Math.floor((Date.now() - enrolledAt) / 1000) + 9;
							const nextTarget = Math.min(secondsNeeded, secondsDone + speed);
							const sendTimestamp = Math.min(secondsNeeded, maxAllowed, nextTarget + Math.random() * 0.4);

							if (sendTimestamp <= secondsDone && secondsDone < secondsNeeded) {
								await new Promise(r => setTimeout(r, 2000));
								continue;
							}

							const res = await api.post({
								url: "/quests/" + quest.id + "/video-progress",
								body: { timestamp: sendTimestamp }
							});

							completed = res.body?.completed_at != null;
							const serverVal = Math.floor(res.body?.progress?.[taskName]?.value ?? nextTarget);
							secondsDone = Math.min(secondsNeeded, Math.max(nextTarget, serverVal));
							state.liveProgress[quest.id] = secondsDone;

							if (completed || secondsDone >= secondsNeeded) break;
						}

						if (state.abort || state.runToken !== token) return resolve();

						if (!completed) {
							await api.post({
								url: "/quests/" + quest.id + "/video-progress",
								body: { timestamp: secondsNeeded }
							});
						}
						state.liveProgress[quest.id] = secondsNeeded;
						addLog("Выполнено: «" + questName + "»", "ok");
						await finish();
					} catch (err) {
						addLog("Ошибка видео «" + questName + "»: " + formatErr(err), "err");
						resolve();
					}
				})();

			} else if (taskName === "PLAY_ON_DESKTOP") {
				api.get({ url: "/applications/public?application_ids=" + applicationId }).then(res => {
					if (state.abort || state.runToken !== token) return resolve();
					const appData = res.body[0];
					const exeName = appData.executables?.find(x => x.os === "win32")?.name?.replace(">", "") ?? appData.name.replace(/[\/\\:*?"<>|]/g, "");

					const fakeGame = {
						cmdLine: "C:\\Program Files\\" + appData.name + "\\" + exeName,
						exeName: exeName,
						exePath: "c:/program files/" + appData.name.toLowerCase() + "/" + exeName,
						hidden: false,
						isLauncher: false,
						id: applicationId,
						name: appData.name,
						pid: pid,
						pidPath: [pid],
						processName: appData.name,
						start: Date.now()
					};

					const realGames = RunningGameStore.getRunningGames();
					const fakeGames = [fakeGame];
					const realGetRunningGames = RunningGameStore.getRunningGames;
					const realGetGameForPID = RunningGameStore.getGameForPID;

					RunningGameStore.getRunningGames = () => fakeGames;
					RunningGameStore.getGameForPID = (p) => fakeGames.find(x => x.pid === p);
					FluxDispatcher.dispatch({ type: "RUNNING_GAMES_CHANGE", removed: realGames, added: [fakeGame], games: fakeGames });

					const restoreGame = () => {
						RunningGameStore.getRunningGames = realGetRunningGames;
						RunningGameStore.getGameForPID = realGetGameForPID;
						FluxDispatcher.dispatch({ type: "RUNNING_GAMES_CHANGE", removed: [fakeGame], added: [], games: realGames });
						FluxDispatcher.unsubscribe("QUESTS_SEND_HEARTBEAT_SUCCESS", fn);
					};

					const fn = data => {
						let progress = quest.config.configVersion === 1
							? data.userStatus.streamProgressSeconds
							: Math.floor(data.userStatus.progress?.PLAY_ON_DESKTOP?.value ?? 0);
						state.liveProgress[quest.id] = progress;

						if (progress >= secondsNeeded || state.runToken !== token) {
							restoreGame();
							if (progress >= secondsNeeded) {
								addLog("Выполнено: «" + questName + "»", "ok");
								finish();
							} else {
								resolve();
							}
						}
					};

					state.cleanupFns.push(restoreGame);
					FluxDispatcher.subscribe("QUESTS_SEND_HEARTBEAT_SUCCESS", fn);
					addLog("Игра «" + appData.name + "» запущена (~" + Math.ceil((secondsNeeded - secondsDone) / 60) + " мин)", "run");
				}).catch(err => {
					addLog("Ошибка игры «" + questName + "»: " + formatErr(err), "err");
					resolve();
				});

			} else if (taskName === "STREAM_ON_DESKTOP") {
				if (!ApplicationStreamingStore) {
					addLog("Модуль стрима недоступен для «" + questName + "»", "err");
					return resolve();
				}
				const realFunc = ApplicationStreamingStore.getStreamerActiveStreamMetadata;
				ApplicationStreamingStore.getStreamerActiveStreamMetadata = () => ({
					id: applicationId,
					pid: pid,
					sourceName: null
				});

				const restoreStream = () => {
					ApplicationStreamingStore.getStreamerActiveStreamMetadata = realFunc;
					FluxDispatcher.unsubscribe("QUESTS_SEND_HEARTBEAT_SUCCESS", fn);
				};

				const fn = data => {
					let progress = quest.config.configVersion === 1
						? data.userStatus.streamProgressSeconds
						: Math.floor(data.userStatus.progress?.STREAM_ON_DESKTOP?.value ?? 0);
					state.liveProgress[quest.id] = progress;

					if (progress >= secondsNeeded || state.runToken !== token) {
						restoreStream();
						if (progress >= secondsNeeded) {
							addLog("Выполнено: «" + questName + "»", "ok");
							finish();
						} else {
							resolve();
						}
					}
				};

				state.cleanupFns.push(restoreStream);
				FluxDispatcher.subscribe("QUESTS_SEND_HEARTBEAT_SUCCESS", fn);
				addLog("Стрим «" + questName + "»: включи демку экрана в войсе (нужен 1+ человек).", "run");

			} else if (taskName === "PLAY_ACTIVITY") {
				const dmId = ChannelStore?.getSortedPrivateChannels?.()[0]?.id;
				const guildWithVoice = Object.values(GuildChannelStore?.getAllGuilds?.() || {}).find(x => x != null && x.VOCAL && x.VOCAL.length > 0);
				const channelId = dmId ?? guildWithVoice?.VOCAL?.[0]?.channel?.id;

				if (!channelId) {
					addLog("Не найден канал для активности: «" + questName + "»", "err");
					return resolve();
				}

				const streamKey = "call:" + channelId + ":1";
				addLog("Активность: «" + questName + "»", "run");

				(async () => {
					try {
						while (!state.abort && state.runToken === token) {
							const res = await api.post({
								url: "/quests/" + quest.id + "/heartbeat",
								body: { stream_key: streamKey, terminal: false }
							});
							const progress = Math.floor(res.body?.progress?.PLAY_ACTIVITY?.value ?? 0);
							state.liveProgress[quest.id] = progress;

							if (progress >= secondsNeeded || res.body?.completed_at != null) {
								await api.post({
									url: "/quests/" + quest.id + "/heartbeat",
									body: { stream_key: streamKey, terminal: true }
								});
								addLog("Выполнено: «" + questName + "»", "ok");
								return await finish();
							}

							await new Promise(r => setTimeout(r, 20000));
						}
						resolve();
					} catch (err) {
						addLog("Ошибка активности «" + questName + "»: " + formatErr(err), "err");
						resolve();
					}
				})();
			} else {
				resolve();
			}
		});
	}

	async function runSingleBg(questId, token) {
		try {
			let quest = QuestsStore.quests.get(questId);
			if (!quest) {
				addLog("Квест не найден.", "err");
				return;
			}

			if (!quest.userStatus?.enrolledAt) {
				const ok = await enrollQuest(questId, quest.config.messages?.questName || questId);
				if (!ok || state.runToken !== token) return;
				quest = await waitForEnrollment(questId, 3000);
			}

			await executeQuest(quest, token);
		} finally {
			if (state.runToken === token) {
				state.isRunning = false;
				state.currentQuestId = null;
			}
		}
	}

	async function runAllBg(token) {
		try {
			if (state.autoEnroll) {
				const enrolledCount = await enrollAllAvailable();
				if (enrolledCount > 0) {
					await new Promise(r => setTimeout(r, 700));
				}
			}

			const doneSet = new Set();
			while (!state.abort && state.runToken === token) {
				const queue = collectEligibleQueue().filter(q => !doneSet.has(q.id));
				if (queue.length === 0) {
					addLog("Все доступные задания выполнены!", "ok");
					break;
				}
				const nextQuest = queue[0];
				doneSet.add(nextQuest.id);
				await executeQuest(nextQuest, token);
			}
		} finally {
			if (state.runToken === token) {
				state.isRunning = false;
				state.currentQuestId = null;
			}
		}
	}

	function getState() {
		readStoreBalance();
		const earned = (typeof state.orbBalance === "number" && typeof state.initialOrbBalance === "number")
			? Math.max(0, state.orbBalance - state.initialOrbBalance)
			: 0;
		return {
			isRunning: state.isRunning,
			autoEnroll: state.autoEnroll,
			autoStart: state.autoStart,
			currentQuestId: state.currentQuestId,
			orbBalance: state.orbBalance,
			earnedOrbs: earned,
			user: getUserInfo(),
			initError: state.initError,
			logs: state.logs,
			quests: parseQuests()
		};
	}

	async function tick() {
		if (state.isTicking) return;
		state.isTicking = true;
		try {
			await refreshBalance(false);
			if (state.autoEnroll && !state.initError) {
				const newlyEnrolled = await enrollAllAvailable();
				if (newlyEnrolled > 0 && state.autoStart && !state.isRunning) {
					window.__questTrackerV9.startAll();
				}
			}
			if (state.autoStart && !state.isRunning && !state.initError) {
				const eligible = collectEligibleQueue();
				if (eligible.length > 0) {
					window.__questTrackerV9.startAll();
				}
			}
		} finally {
			state.isTicking = false;
		}
	}

	window.__questTrackerV9 = {
		startAll: function() {
			if (state.initError) {
				addLog("Ошибка: " + state.initError, "err");
				return JSON.stringify(getState());
			}
			stopInternal(true);
			const token = ++state.runToken;
			state.abort = false;
			state.isRunning = true;
			addLog("Запущено выполнение всех доступных квестов.", "run");
			runAllBg(token);
			return JSON.stringify(getState());
		},
		startOne: function(questId) {
			if (state.initError) {
				addLog("Ошибка: " + state.initError, "err");
				return JSON.stringify(getState());
			}
			stopInternal(true);
			const token = ++state.runToken;
			state.abort = false;
			state.isRunning = true;
			state.currentQuestId = questId;
			const q = QuestsStore.quests.get(questId);
			addLog("Запуск квеста: «" + (q?.config?.messages?.questName || questId) + "»", "run");
			runSingleBg(questId, token);
			return JSON.stringify(getState());
		},
		stop: function() {
			stopInternal(false);
			return JSON.stringify(getState());
		},
		enrollNow: function() {
			addLog("Проверка и принятие новых квестов...", "info");
			enrollAllAvailable().then((cnt) => {
				if (cnt === 0) addLog("Новых непринятых квестов нет.", "info");
			});
			return JSON.stringify(getState());
		},
		setSettings: function(autoEnroll, autoStart) {
			if (typeof autoEnroll === "boolean") {
				state.autoEnroll = autoEnroll;
				addLog("Автопринятие: " + (autoEnroll ? "включено" : "выключено"), "info");
			}
			if (typeof autoStart === "boolean") {
				state.autoStart = autoStart;
				addLog("Автозапуск: " + (autoStart ? "включён" : "выключен"), "info");
			}
			tick();
			return JSON.stringify(getState());
		},
		getState: getState
	};

	addLog("Контроллер подключен к Discord.", "ok");
	refreshBalance(true);
	tick();
	window.__questTrackerV9Timer = setInterval(tick, 12000);
	return JSON.stringify(getState());
})();