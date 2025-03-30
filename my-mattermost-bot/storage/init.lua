box.cfg{
    listen= 3301
}

box.schema.space.create('votings', {if_not_exists = true})
box.space.votings:format({
    {name = 'id', type = 'string'},
    {name = 'question', type = 'string'},
    {name = 'options', type = 'map'},
    {name = 'created_by', type = 'string'},
    {name = 'is_closed', type = 'boolean'},
})

box.space.votings:create_index('primary',{
    parts = {'id'},
    if_not_exists = true
})