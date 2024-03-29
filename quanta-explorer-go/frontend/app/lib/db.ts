import mongoose from 'mongoose';

const url = process.env.DATABASE_URL as string;
let connection: typeof mongoose;

const startDb = async () => {
    if (!connection) connection = await mongoose.connect(url);
    return connection;
};

export default startDb;