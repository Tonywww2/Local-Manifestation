// Original content uses the public scripting contract; no engine state is exposed.
sdk.registerBehavior("base:chase", function(ctx) {
    return {
        move: {x: ctx.target.x - ctx.self.x, y: ctx.target.y - ctx.self.y},
        attack: ctx.distance < 40
    };
});

// Ranged forms approach mid-distance, then reposition without perpetual retreat.
sdk.registerBehavior("base:midrange", function(ctx) {
 return {move:ctx.distance>330 ? {x:ctx.target.x-ctx.self.x,y:ctx.target.y-ctx.self.y} : {x:0,y:0},attack:false};
});

sdk.registerNode("base:forge", {
    cost_per_level: {type: "integer", minimum: 1, maximum: 100000},
    damage_per_upgrade: {type: "number", minimum: 0, maximum: 100000}
}, function(ctx) {
    if (ctx.command !== "upgrade") return [];
    return [
        {kind: "spend_coins", target: ctx.player.id, amount: ctx.parameters.cost_per_level * (ctx.player.upgrade + 1)},
        {kind: "upgrade_weapon", target: ctx.player.id, amount: 1},
        {kind: "weapon_bonus", target: ctx.player.id, amount: ctx.parameters.damage_per_upgrade}
    ];
});

sdk.registerNode("base:rest", {
    penalty_removal: {type: "integer", minimum: 0, maximum: 3}
}, function(ctx) {
    if (ctx.command !== "enter" || !ctx.first_visit) return [];
    return [
        {kind: "remove_penalty", target: ctx.player.id, amount: ctx.parameters.penalty_removal},
        {kind: "heal_full", target: ctx.player.id, amount: 0}
    ];
});

// An inert original listener exercises the same lifecycle used by extensions.
sdk.on("base:wave-observer", "scene", "wave_completed", function(ctx) {
    return [];
});
// Development hit content. All item sources use a stable instance ID.
sdk.on("base:ember-hit", "scene", "hit", function(ctx) {
    var p=ctx.players.find(function(p){return p.id===ctx.event.source;});
    if(!p || !ctx.event.attack) return [];
    var out=[];
    p.inventory.items.forEach(function(it){
        if(p.inventory.accessories.indexOf(it.id)>=0 && it.definition==="base:ember-charm")
            out.push({kind:"apply_status",target:ctx.event.target,status:"base:burn",amount:0,source:{kind:"equipment",actor:p.id,definition:it.definition,instance:it.id}});
    }); return out;
});
sdk.on("base:drain-after", "scene", "after_damage", function(ctx) {
    var p=ctx.players.find(function(p){return p.id===ctx.event.source;});
    if(!p || !ctx.event.attack) return [];
    var it=p.inventory.items.find(function(it){return p.inventory.accessories.indexOf(it.id)>=0 && it.definition==="base:drain-charm";});
    if(!it) return [];
    return [{kind:"lifesteal",target:p.id,amount:.03,source:{kind:"equipment",actor:p.id,definition:it.definition,instance:it.id}}];
});
// Cleansing development event. Entry grants one owned consumable; use removes the curse.
sdk.registerNode("base:cleanse-node", {}, function(ctx) {
    if(ctx.command!=="enter" || !ctx.first_visit) return [];
    return [{kind:"grant_item",target:ctx.player.id,item:"base:cleanse",amount:1}];
});
